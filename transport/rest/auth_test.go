package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
	"github.com/buzkaaclicker/buzza/inmem"
	"github.com/buzkaaclicker/buzza/persistent"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/buntdb"
)

func Test_AuthLoginLogoutFlow(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})

	bunt, err := buntdb.Open(":memory:")
	if !assert.NoError(err) {
		return
	}
	defer func () {
		_= bunt.Close()
	}()

	userStore := inmem.NewUserStore()
	activityStore := inmem.NewActivityStore()
	authController := AuthController{
		UserStore:      &userStore,
		SessionStore:   &persistent.SessionStore{
			Buntdb:        bunt,
			ActivityStore: &activityStore,
		},
		GuildMemberAdd: discord.MockGuildMemberAdd,
	}
	authController.InstallTo(app)

	type Case struct {
		AccessTokenExchangeErr error
		User                   discord.User
		UserMeErr              error
		Validate               func(resp *http.Response, body string)
		StatusCode             int
	}

	properUser := discord.User{Email: "e@ma.il", Username: "no email access", Id: "928592940128"}

	validateInternalError := func(resp *http.Response, body string) {
		assert.Equal(JsonErrorMessageResponse(fiber.ErrInternalServerError.Message), body)
	}

	validateCreated := func(resp *http.Response, body string) {
		assert.Equal(resp.Header.Get("Content-Type"), fiber.MIMEApplicationJSON, "Invalid content type")

		user, err := userStore.ByDiscordId(ctx, properUser.Id)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(user.Discord.Id, properUser.Id)
		assert.Equal(string(user.Email), properUser.Email)

		logs, err := activityStore.ByUserId(ctx, user.Id)
		if !assert.NoError(err) || !assert.GreaterOrEqual(len(logs), 1) {
			return
		}
		assert.Equal("session_created", logs[len(logs)-1].Name)
	}

	cases := []Case{
		{
			Validate: func(resp *http.Response, body string) {
				assert.Equal(JsonErrorMessageResponse("invalid code"), body)
			},
			User:                   properUser,
			AccessTokenExchangeErr: discord.ErrOAuthInvalidCode,
			StatusCode:             fiber.StatusUnauthorized,
		},
		{
			Validate:               validateInternalError,
			User:                   properUser,
			AccessTokenExchangeErr: errors.New("unexpected error"),
			StatusCode:             fiber.StatusInternalServerError,
		},
		{
			Validate:   validateInternalError,
			User:       properUser,
			UserMeErr:  errors.New("unexpected error"),
			StatusCode: fiber.StatusInternalServerError,
		},
		{
			Validate:   validateInternalError,
			User:       properUser,
			UserMeErr:  discord.ErrUnauthorized,
			StatusCode: fiber.StatusInternalServerError,
		},
		{
			Validate: func(resp *http.Response, body string) {
				assert.Equal(JsonErrorMessageResponse("missing email"), body)
			},
			User:       discord.User{Username: "no email access", Id: "2222"},
			StatusCode: fiber.StatusBadRequest},
		{
			Validate:   validateCreated,
			User:       properUser,
			StatusCode: fiber.StatusCreated,
		},
	}

	// returns accessToken on success, otherwise empty string
	testLogin := func(tc Case) string {
		t.Logf("Case: %v\n", tc)
		req := httptest.NewRequest("POST", "/auth/discord", bytes.NewBuffer([]byte(`{"code": "21"}`)))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req)
		if !assert.NoError(err) {
			return ""
		}
		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if !assert.NoError(err) {
			return ""
		}
		body := string(bodyBytes)
		tc.Validate(resp, body)

		if !assert.Equal(tc.StatusCode, resp.StatusCode) {
			return ""
		}

		if resp.StatusCode/100 == 2 {
			type Response struct {
				AccessToken string `json:"accessToken"`
			}
			response := new(Response)
			err := json.Unmarshal(bodyBytes, response)
			if !assert.NoError(err) {
				return ""
			}
			sessionExists, err := authController.SessionStore.Exists(response.AccessToken)
			if !assert.NoError(err) {
				return ""
			}
			assert.True(sessionExists)

			return response.AccessToken
		} else {
			return ""
		}
	}

	testLogout := func(tc Case, accessToken string) {
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+accessToken)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(fiber.StatusOK, resp.StatusCode)
		sessionExists, err := authController.SessionStore.Exists(accessToken)
		if !assert.NoError(err) {
			return
		}
		assert.False(sessionExists)
	}

	caseTest := func(tc Case) {
		authController.ExchangeAccessToken = func(code string) (discord.AccessTokenResponse, error) {
			return discord.AccessTokenResponse{}, tc.AccessTokenExchangeErr
		}
		authController.UserMeProvider = func() discord.UserMe {
			return func(token discord.Token) (discord.User, error) {
				return tc.User, tc.UserMeErr
			}
		}

		accessToken := testLogin(tc)
		if accessToken != "" {
			testLogout(tc, accessToken)
		}
	}

	for _, tc := range cases {
		caseTest(tc)
	}
}

func Test_SessionAuthorization(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	ctx := context.Background()
	assert := assert.New(t)

	restrictedHandler := func(ctx *fiber.Ctx) error {
		session := ctx.Locals(sessionLocalsKey).(buzza.Session)
		_, err := fmt.Fprintf(ctx, "Authorized. User id: %d", session.UserId)
		return err
	}

	bdb, err := buntdb.Open(":memory:")
	if err != nil {
		panic(err)
	}
	userStore := inmem.NewUserStore()
	activityStore := inmem.NewActivityStore()
	sessionStore := &persistent.SessionStore{
		Buntdb:        bdb,
		ActivityStore: &activityStore,
	}
	controller := AuthController{
		UserStore: &userStore,
	}

	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	requestAuthorizer := RequestAuthorizer(sessionStore, &userStore)
	app.Get("/test/restricted", combineHandlers(requestAuthorizer, restrictedHandler))
	app.Get("/test/dashboard", combineHandlers(requestAuthorizer, requirePermissions(buzza.PermissionAdminDashboard), restrictedHandler))

	registerUser := func(discordUser discord.User) (buzza.User, buzza.Session, error) {
		user, err := userStore.RegisterDiscordUser(context.Background(), discordUser, "refresh-token-mock")
		if err != nil {
			return buzza.User{}, buzza.Session{}, fmt.Errorf("register user: %w", err)
		}
		session, err := sessionStore.RegisterNew(ctx, user.Id, "127.0.0.1", "Safari (Iphone 16 256gb space gray)")
		if err != nil {
			return buzza.User{}, buzza.Session{}, fmt.Errorf("register session: %w", err)
		}
		return user, session, nil
	}

	unprivilegedUser, unprivilegedSession, err := registerUser(
		discord.User{Id: "makin", Username: "makin", Email: "makin"})
	if !assert.NoError(err) {
		return
	}

	privilegedUser, privilegedSession, err := registerUser(
		discord.User{Id: "morton", Username: "morton", Email: "morton"})
	if !assert.NoError(err) {
		return
	}
	privilegedUser.Roles = append(privilegedUser.Roles, buzza.AllRoles[buzza.RoleIdAdmin])
	err = userStore.Update(ctx, privilegedUser)
	if !assert.NoError(err) {
		return
	}

	type Case struct {
		path             string
		token            string
		tokenType        string
		expectedResponse string
	}
	cases := []Case{
		{
			path:             "/test/restricted",
			token:            unprivilegedSession.Token,
			tokenType:        "Bearer",
			expectedResponse: "Authorized. User id: " + strconv.Itoa(int(unprivilegedUser.Id)),
		},
		{
			path:             "/test/restricted",
			token:            "",
			expectedResponse: JsonErrorMessageResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/restricted",
			token:            "unexisting_session_token",
			tokenType:        "Bearer",
			expectedResponse: JsonErrorMessageResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/restricted",
			token:            "basic_is_not_a_valid_auth_type",
			tokenType:        "Basic",
			expectedResponse: JsonErrorMessageResponse("invalid auth type"),
		},
		// permission cases
		{
			path:             "/test/dashboard",
			token:            unprivilegedSession.Token,
			tokenType:        "Bearer",
			expectedResponse: JsonErrorMessageResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/dashboard",
			token:            "",
			expectedResponse: JsonErrorMessageResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/dashboard",
			token:            "not existing token",
			expectedResponse: JsonErrorMessageResponse("invalid auth type"),
		},
		{
			path:             "/test/dashboard",
			token:            privilegedSession.Token,
			tokenType:        "Bearer",
			expectedResponse: "Authorized. User id: " + strconv.Itoa(int(privilegedUser.Id)),
		},
	}

	controller.ExchangeAccessToken = func(code string) (discord.AccessTokenResponse, error) {
		return discord.AccessTokenResponse{RefreshToken: "mock_refresh_token"}, nil
	}

	caseTest := func(tc Case) {
		req := httptest.NewRequest("GET", tc.path, nil)
		if tc.token != "" {
			req.Header.Set("Authorization", tc.tokenType+" "+tc.token)
		}
		resp, err := app.Test(req)
		if !assert.NoError(err) {
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(tc.expectedResponse, string(body), tc)
	}
	for _, tc := range cases {
		caseTest(tc)
	}
}

