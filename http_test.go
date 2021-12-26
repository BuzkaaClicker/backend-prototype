package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func fakeHttpErrorResponse(message string) string {
	bytes, err := json.Marshal(ErrorResponse{ErrorMessage: message})
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

func TestNotFoundHandler(t *testing.T) {
	assert := assert.New(t)

	app := fiber.New(fiber.Config{
		ErrorHandler: restErrorHandler,
	})
	app.Get("/home", func(ctx *fiber.Ctx) error {
		return ctx.SendString(`{"im":"working"}`)
	})
	app.Use(notFoundHandler)

	cases := []struct {
		path       string
		returnCode int
		returnBody string
	}{
		{path: "/unknown_path", returnCode: fiber.StatusNotFound,
			returnBody: fakeHttpErrorResponse("Not Found")},
		{path: "/home", returnCode: fiber.StatusOK,
			returnBody: `{"im":"working"}`},
	}

	for _, useCase := range cases {
		assertMsg := "status code: " + useCase.path

		req := httptest.NewRequest("GET", useCase.path, nil)
		resp, err := app.Test(req)
		assert.NoError(err, assertMsg)
		defer resp.Body.Close()

		assert.Equal(useCase.returnCode, resp.StatusCode, assertMsg)
		body, err := ioutil.ReadAll(resp.Body)
		assert.NoError(err, assertMsg)
		assert.Equal(useCase.returnBody, string(body), assertMsg)
	}
}
