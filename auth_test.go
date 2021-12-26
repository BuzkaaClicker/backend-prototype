package main

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buzkaaclicker/backend/discord"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestAuthCreateUser(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	_, err := db.NewCreateTable().
		IfNotExists().
		Model((*User)(nil)).
		Exec(ctx)
	assert.NoError(err)

	type Case struct {
		AccessTokenResponse    discord.AccessTokenResponse
		AccessTokenExchangeErr error
		User                   discord.User
		UserMeErr              error
		Validate               func(resp *http.Response, body string)
	}

	properUser := discord.User{Email: "e@ma.il", Username: "no email access", Id: "928592940128"}

	// :D EXPECTED ALBO NIE bool == int1 :D
	validateEntitiesCount := func(expectedCount bool) {
		var users []User
		err = db.NewSelect().
			Model((*User)(nil)).
			Where("discord_id=?", properUser.Id).
			Scan(ctx, &users)
		assert.NoError(err)

		if expectedCount {
			assert.Equal(1, len(users))

			user := users[0]
			assert.Equal(user.DiscordId, properUser.Id)
			assert.Equal(user.Email, properUser.Email)
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
		err = db.NewSelect().
			Model((*User)(nil)).
			Where("discord_id=?", properUser.Id).
			Scan(ctx, &users)
		assert.NoError(err)
		assert.Equal(1, len(users))
		user := users[0]
		assert.Equal(user.DiscordId, properUser.Id)
		assert.Equal(user.Email, properUser.Email)
	}

	cases := []Case{
		{Validate: validateOAuthCode, User: properUser, AccessTokenExchangeErr: discord.ErrOAuthInvalidCode},
		{Validate: validateInternalError, User: properUser, AccessTokenExchangeErr: errors.New("unexpected error")},
		{Validate: validateInternalError, User: properUser, UserMeErr: errors.New("unexpected error")},
		{Validate: validateInternalError, User: properUser, UserMeErr: discord.ErrUnauthorized},
		{Validate: validateMail, User: discord.User{Username: "no email access", Id: "2222"}},
		{Validate: validateCreated, User: properUser},
	}

	for _, tc := range cases {
		controller := AuthController{
			DB: db,
			UserMeProvider: func() discord.UserMe {
				return func(tokenType, token string) (discord.User, error) {
					return tc.User, tc.UserMeErr
				}
			},
			AccessTokenExchange: func(code string) (discord.AccessTokenResponse, error) {
				return tc.AccessTokenResponse, tc.AccessTokenExchangeErr
			},
		}

		app := fiber.New(fiber.Config{
			ErrorHandler: restErrorHandler,
		})
		app.Get("/auth/discord", controller.LoginDiscord)

		req := httptest.NewRequest("GET", "/auth/discord?code=21", nil)
		resp, err := app.Test(req)
		assert.NoError(err)
		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		assert.NoError(err)
		body := string(bodyBytes)

		tc.Validate(resp, body)
	}
}
