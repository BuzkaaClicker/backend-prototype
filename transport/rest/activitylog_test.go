package rest

import (
	"context"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/mock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestActivityController(t *testing.T) {
	store := &mock.ActivityStore{
		ByUserIdFn: func(ctx context.Context, userId buzza.UserId) ([]buzza.ActivityLog, error) {
			return []buzza.ActivityLog{
				{
					Id:        1,
					CreatedAt: time.Date(2022, 1, 1, 15, 0, 0, 0, time.UTC),
					UserId:    22,
					Name:      "logged_in",
					Data: map[string]interface{}{
						"ip":        "127.0.0.1",
						"UserAgent": "Safari Iphon 16 Gold Rose 16gb",
					},
				},
				{
					Id:        2,
					CreatedAt: time.Date(2022, 1, 1, 16, 0, 0, 0, time.UTC),
					UserId:    22,
					Name:      "logged_out",
					Data: map[string]interface{}{
						"ip": "127.0.0.1",
					},
				},
				{
					Id:        2,
					CreatedAt: time.Date(2022, 1, 1, 16, 5, 0, 0, time.UTC),
					UserId:    22,
					Name:      "click_clack",
				},
			}, nil
		},
	}

	app := fiber.New()
	controller := ActivityController{
		Store: store,
	}
	controller.InstallTo(func(ctx *fiber.Ctx) error {
		ctx.Locals(userLocalsKey, &buzza.User{Id: 2})
		return nil
	}, app)

	req := httptest.NewRequest("GET", "/activities", nil)
	resp, err := app.Test(req)
	if !assert.NoError(t, err) {
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, `[{"id":1,"createdAt":1641049200,"name":"logged_in","data":{"UserAgent":"Safari Iphon 16 Gold Rose 16gb","ip":"127.0.0.1"}},`+
		`{"id":2,"createdAt":1641052800,"name":"logged_out","data":{"ip":"127.0.0.1"}},`+
		`{"id":2,"createdAt":1641053100,"name":"click_clack"}]`,
		string(body))
}

