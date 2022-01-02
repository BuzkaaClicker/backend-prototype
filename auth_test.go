package main

import (
	"context"
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

func Test_AuthCreateUser(t *testing.T) {
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
		assert.Equal(fiber.StatusBadRequest, resp.StatusCode)
		assert.Equal(fakeHttpErrorResponse("missing email"), body)
	}

	validateOAuthCode := func(resp *http.Response, body string) {
		validateEntitiesCount(false)
		assert.Equal(fiber.StatusBadRequest, resp.StatusCode)
		assert.Equal(fakeHttpErrorResponse("invalid code"), body)
	}

	validateInternalError := func(resp *http.Response, body string) {
		validateEntitiesCount(false)
		assert.Equal(fiber.StatusInternalServerError, resp.StatusCode)
		assert.Equal(fakeHttpErrorResponse(fiber.ErrInternalServerError.Message), body)
	}

	validateCreated := func(resp *http.Response, body string) {
		validateEntitiesCount(true)

		assert.Equal(fiber.StatusCreated, resp.StatusCode)
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
		{Validate: validateOAuthCode, User: properUser, AccessTokenExchangeErr: discord.ErrOAuthInvalidCode},
		{Validate: validateInternalError, User: properUser, AccessTokenExchangeErr: errors.New("unexpected error")},
		{Validate: validateInternalError, User: properUser, UserMeErr: errors.New("unexpected error")},
		{Validate: validateInternalError, User: properUser, UserMeErr: discord.ErrUnauthorized},
		{Validate: validateMail, User: discord.User{Username: "no email access", Id: "2222"}},
		{Validate: validateCreated, User: properUser},
	}

	caseTest := func (tc Case) {
		app.authController.AccessTokenExchanger = func(code string) (discord.AccessTokenResponse, error) {
			return discord.AccessTokenResponse{}, tc.AccessTokenExchangeErr
		}
		app.authController.UserMeProvider = func() discord.UserMe {
			return func(tokenType, token string) (discord.User, error) {
				return tc.User, tc.UserMeErr
			}
		}

		req := httptest.NewRequest("GET", "/auth/discord?code=21", nil)
		resp, err := app.server.Test(req)
		if !assert.NoError(err) {
			return
		}
		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if !assert.NoError(err) {
			return
		}
		body := string(bodyBytes)

		tc.Validate(resp, body)
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

	app := createTestApp()

	discordUser := discord.User{Username: "makin", Email: "makin"}
	user, err := app.userStore.RegisterDiscordUser(context.Background(), discordUser, "empty")
	if !assert.NoError(err) {
		return
	}

	session, err := app.sessionStore.RegisterNew(user.Id)
	if !assert.NoError(err) {
		return
	}
	assert.NotNil(session)

	restrictedHandler := func(ctx *fiber.Ctx) error {
		session := ctx.Locals(SessionKey).(*Session)
		_, err := fmt.Fprintf(ctx, "Authorized. User id: %d", session.UserId)
		return err
	}

	app.server.Get("/restricted", combineHandlers(app.sessionStore.Authorize, restrictedHandler))

	type Case struct {
		token            string
		tokenType        string
		expectedResponse string
	}
	cases := []Case{
		{
			token:            session.Token,
			tokenType:        "Bearer",
			expectedResponse: "Authorized. User id: " + strconv.Itoa(int(user.Id)),
		},
		{
			token:            "",
			expectedResponse: "Unauthorized",
		},
		{
			token:            "unexisting_session_token",
			tokenType:        "Bearer",
			expectedResponse: "Unauthorized",
		},
		{
			token:            "basic_is_not_a_valid_auth_type",
			tokenType:        "Basic",
			expectedResponse: "invalid auth type",
		},
	}

	app.authController.AccessTokenExchanger = func(code string) (discord.AccessTokenResponse, error) {
		return discord.AccessTokenResponse{RefreshToken: "mock_refresh_token"}, nil
	}
	app.authController.UserMeProvider = func() discord.UserMe {
		return func(tokenType, token string) (discord.User, error) {
			return discordUser, nil
		}
	}

	caseTest := func(tc Case) {
		req := httptest.NewRequest("GET", "/restricted", nil)
		if tc.token != "" {
			req.Header.Set("Authorization", tc.tokenType+" "+tc.token)
		}
		resp, err := app.server.Test(req)
		if !assert.NoError(err) {
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if assert.NoError(err) {
			return
		}
		assert.Equal(tc.expectedResponse, string(body), tc)
	}
	for _, tc := range cases {
		caseTest(tc)
	}
}
