package rest

import (
	"context"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/mock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestProfileControllerLookup(t *testing.T) {
	assert := assert.New(t)

	controller := ProfileController{
		Store: mock.ProfileService{
			ByUserIdFn: func(ctx context.Context, userId buzza.UserId) (buzza.Profile, error) {
				return buzza.Profile{
					User:      buzza.User{Id: 1},
					Name:      "ww_makin_c",
					AvatarUrl: "https://buzkaaclicker.pl/avatar/123",
				}, nil
			},
		},
	}
	app := fiber.New()
	controller.InstallTo(app)

	req := httptest.NewRequest("GET", "/profile/1", nil)
	resp, err := app.Test(req)
	if !assert.NoError(err) {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(`{"name":"ww_makin_c","avatarUrl":"https://buzkaaclicker.pl/avatar/123"}`, string(body))
}
