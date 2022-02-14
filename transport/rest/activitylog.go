package rest

import (
	"fmt"

	"github.com/buzkaaclicker/buzza"
	"github.com/gofiber/fiber/v2"
)

type ActivityController struct {
	Store buzza.ActivityStore
}

func (c *ActivityController) InstallTo(authorizationHandler fiber.Handler, app *fiber.App) {
	app.Get("/activities", c.lastActivityHandler(authorizationHandler))
}

func (c *ActivityController) lastActivityHandler(authorizationHandler fiber.Handler) fiber.Handler {
	return combineHandlers(authorizationHandler, c.serveLastActivity)
}

func (c *ActivityController) serveLastActivity(ctx *fiber.Ctx) error {
	user, ok := ctx.Locals(userLocalsKey).(*buzza.User)
	if !ok {
		return fiber.ErrUnauthorized
	}
	logs, err := c.Store.ByUserId(ctx.Context(), user.Id)
	if err != nil {
		return fmt.Errorf("get logs by user id: %w", err)
	}

	type Log struct {
		Id        int64                  `json:"id"`
		CreatedAt int64                  `json:"createdAt"`
		Name      string                 `json:"name"`
		Data      map[string]interface{} `json:"data,omitempty"`
	}
	mapped := make([]Log, len(logs))
	for i, log := range logs {
		mapped[i] = Log{Id: log.Id, CreatedAt: log.CreatedAt.Unix(), Name: log.Name, Data: log.Data}
	}
	return ctx.JSON(mapped)
}
