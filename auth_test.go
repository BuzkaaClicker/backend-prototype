package main

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

	"github.com/buzkaaclicker/backend/discord"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func Test_AuthLoginLogoutFlow(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	app := createTestApp()

	type Case struct {
		AccessTokenExchangeErr error
		User                   discord.User
		UserMeErr              error
		Validate               func(resp *http.Response, body string)
		StatusCode             int
	}

	properUser := discord.User{Email: "e@ma.il", Username: "no email access", Id: "928592940128"}

	// :D EXPECTED ALBO NIE bool == int1 :D
	validateEntitiesCount := func(expectedCount bool) {
		var users []User
		err := app.db.NewSelect().
			Model((*User)(nil)).
			Where("discord_id=?", properUser.Id).
			Scan(ctx, &users)
		if !assert.NoError(err) {
			return
		}

		if expectedCount {
			if assert.Equal(1, len(users)) {
				user := users[0]
				assert.Equal(user.DiscordId, properUser.Id)
				assert.Equal(user.Email, properUser.Email)
			}
		} else {
			assert.Equal(0, len(users))
		}
	}

	validateMail := func(resp *http.Response, body string) {
		validateEntitiesCount(false)
		assert.Equal(fakeHttpErrorResponse("missing email"), body)
	}

	validateOAuthCode := func(resp *http.Response, body string) {
		validateEntitiesCount(false)
		assert.Equal(fakeHttpErrorResponse("invalid code"), body)
	}

	validateInternalError := func(resp *http.Response, body string) {
		validateEntitiesCount(false)
		assert.Equal(fakeHttpErrorResponse(fiber.ErrInternalServerError.Message), body)
	}

	validateCreated := func(resp *http.Response, body string) {
		validateEntitiesCount(true)
		assert.Equal(resp.Header.Get("Content-Type"), fiber.MIMEApplicationJSON, "Invalid content type")

		var users []User
		err := app.db.NewSelect().
			Model((*User)(nil)).
			Where("discord_id=?", properUser.Id).
			Scan(ctx, &users)
		if !assert.NoError(err) {
			return
		}
		if assert.Equal(1, len(users)) {
			user := users[0]
			assert.Equal(user.DiscordId, properUser.Id)
			assert.Equal(user.Email, properUser.Email)
		}
	}

	cases := []Case{
		{Validate: validateOAuthCode, User: properUser, AccessTokenExchangeErr: discord.ErrOAuthInvalidCode,
			StatusCode: fiber.StatusUnauthorized},
		{Validate: validateInternalError, User: properUser, AccessTokenExchangeErr: errors.New("unexpected error"),
			StatusCode: fiber.StatusInternalServerError},
		{Validate: validateInternalError, User: properUser, UserMeErr: errors.New("unexpected error"),
			StatusCode: fiber.StatusInternalServerError},
		{Validate: validateInternalError, User: properUser, UserMeErr: discord.ErrUnauthorized,
			StatusCode: fiber.StatusInternalServerError},
		{Validate: validateMail, User: discord.User{Username: "no email access", Id: "2222"},
			StatusCode: fiber.StatusBadRequest},
		{Validate: validateCreated, User: properUser, StatusCode: fiber.StatusCreated},
	}

	// returns accessToken on success, otherwise empty string
	testLogin := func(tc Case) string {
		req := httptest.NewRequest("POST", "/api/auth/discord", bytes.NewBuffer([]byte(`{"code": "21"}`)))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.server.Test(req)
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
			sessionExists, err := app.authController.SessionStore.Exists(response.AccessToken)
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
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+accessToken)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.server.Test(req)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(fiber.StatusOK, resp.StatusCode)
		sessionExists, err := app.authController.SessionStore.Exists(accessToken)
		if !assert.NoError(err) {
			return
		}
		assert.False(sessionExists)
	}

	caseTest := func(tc Case) {
		app.authController.ExchangeAccessToken = func(code string) (discord.AccessTokenResponse, error) {
			return discord.AccessTokenResponse{}, tc.AccessTokenExchangeErr
		}
		app.authController.UserMeProvider = func() discord.UserMe {
			return func(tokenType, token string) (discord.User, error) {
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

func Test_GenerateSessionTokenLength(t *testing.T) {
	assert := assert.New(t)

	token, err := generateSessionToken()
	if assert.NoError(err) {
		assert.True(len(token) > 20)
	}
}

func Test_SessionAuthorization(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)

	restrictedHandler := func(ctx *fiber.Ctx) error {
		session := ctx.Locals(SessionKey).(*Session)
		_, err := fmt.Fprintf(ctx, "Authorized. User id: %d", session.UserId)
		return err
	}

	app := createTestApp(func(app *app) {
		app.server.Get("/test/restricted", combineHandlers(app.sessionStore.Authorize, restrictedHandler))
		app.server.Get("/test/dashboard", combineHandlers(app.sessionStore.Authorize, RequirePermissions(PermissionAdminDashboard),
			restrictedHandler))
	})

	registerUser := func(discordUser discord.User) (*User, *Session, error) {
		user, err := app.userStore.RegisterDiscordUser(context.Background(), discordUser, "refresh-token-mock")
		if err != nil {
			return nil, nil, fmt.Errorf("register user: %w", err)
		}
		session, err := app.sessionStore.RegisterNew(user.Id)
		if err != nil {
			return nil, nil, fmt.Errorf("register session: %w", err)
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
	privilegedUser.RolesNames = append(privilegedUser.RolesNames, RoleIdAdmin)
	_, err = app.db.NewUpdate().
		Model(privilegedUser).
		Where("id=?", privilegedUser.Id).
		Exec(context.Background())
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
			expectedResponse: fakeHttpErrorResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/restricted",
			token:            "unexisting_session_token",
			tokenType:        "Bearer",
			expectedResponse: fakeHttpErrorResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/restricted",
			token:            "basic_is_not_a_valid_auth_type",
			tokenType:        "Basic",
			expectedResponse: fakeHttpErrorResponse("invalid auth type"),
		},
		// permission cases
		{
			path:             "/test/dashboard",
			token:            unprivilegedSession.Token,
			tokenType:        "Bearer",
			expectedResponse: fakeHttpErrorResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/dashboard",
			token:            "",
			expectedResponse: fakeHttpErrorResponse(fiber.ErrUnauthorized.Message),
		},
		{
			path:             "/test/dashboard",
			token:            "not existing token",
			expectedResponse: fakeHttpErrorResponse("invalid auth type"),
		},
		{
			path:             "/test/dashboard",
			token:            privilegedSession.Token,
			tokenType:        "Bearer",
			expectedResponse: "Authorized. User id: " + strconv.Itoa(int(privilegedUser.Id)),
		},
	}

	app.authController.ExchangeAccessToken = func(code string) (discord.AccessTokenResponse, error) {
		return discord.AccessTokenResponse{RefreshToken: "mock_refresh_token"}, nil
	}

	caseTest := func(tc Case) {
		req := httptest.NewRequest("GET", tc.path, nil)
		if tc.token != "" {
			req.Header.Set("Authorization", tc.tokenType+" "+tc.token)
		}
		resp, err := app.server.Test(req)
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
