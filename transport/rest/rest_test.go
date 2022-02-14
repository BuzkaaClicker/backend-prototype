package rest

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestNotFoundHandler(t *testing.T) {
	assert := assert.New(t)

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})
	app.Get("/home", func(ctx *fiber.Ctx) error {
		return ctx.SendString(`{"im":"working"}`)
	})
	app.Use(NotFoundHandler)

	cases := []struct {
		path       string
		returnCode int
		returnBody string
	}{
		{path: "/unknown_path", returnCode: fiber.StatusNotFound,
			returnBody: JsonErrorMessageResponse("Not Found")},
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
