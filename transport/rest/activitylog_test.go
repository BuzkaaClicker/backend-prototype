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
		ByUserIdFn: func(ctx context.Context, userId buzza.UserId, beforeId int64, limit int32) ([]buzza.ActivityLog, error) {
			allLogs := []buzza.ActivityLog{
				{
					Id:        3,
					CreatedAt: time.Date(2022, 1, 1, 16, 5, 0, 0, time.UTC),
					UserId:    22,
					Name:      "click_clack",
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
					Id:        1,
					CreatedAt: time.Date(2022, 1, 1, 15, 0, 0, 0, time.UTC),
					UserId:    22,
					Name:      "logged_in",
					Data: map[string]interface{}{
						"ip":        "127.0.0.1",
						"UserAgent": "Safari Iphon 16 Gold Rose 16gb",
					},
				},
			}
			if beforeId >= 0 && beforeId <= 2 {
				return allLogs[:2], nil
			}
			return allLogs, nil
		},
	}

	app := fiber.New()
	controller := ActivityController{
		Store: store,
	}
	controller.InstallTo(func(ctx *fiber.Ctx) error {
		ctx.Locals(userLocalsKey, buzza.User{Id: 22})
		return nil
	}, app)

	tcs := []struct {
		url      string
		response string
	}{
		{url: "/activities", response: `[` +
			`{"id":3,"createdAt":1641053100,"name":"click_clack"},` +
			`{"id":2,"createdAt":1641052800,"name":"logged_out","data":{"ip":"127.0.0.1"}},` +
			`{"id":1,"createdAt":1641049200,"name":"logged_in","data":{"UserAgent":"Safari Iphon 16 Gold Rose 16gb","ip":"127.0.0.1"}}]`},

		{url: "/activities?before=2", response: `[` +
			`{"id":3,"createdAt":1641053100,"name":"click_clack"},` +
			`{"id":2,"createdAt":1641052800,"name":"logged_out","data":{"ip":"127.0.0.1"}}` +
			`]`},
	}

	for _, tc := range tcs {
		req := httptest.NewRequest("GET", tc.url, nil)
		resp, err := app.Test(req)
		if !assert.NoError(t, err) {
			return
		}
		body, err := ioutil.ReadAll(resp.Body)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, tc.response, string(body))
	}
}
