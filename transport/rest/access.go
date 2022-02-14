package rest

import (
	"github.com/buzkaaclicker/buzza"
	"github.com/gofiber/fiber/v2"
)

func requirePermissions(permission buzza.PermissionName) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		user, ok := ctx.Locals(userLocalsKey).(buzza.User)
		if !ok {
			return fiber.ErrUnauthorized
		}
		if user.Roles.Access(permission) != buzza.AccessAllowed {
			return fiber.ErrUnauthorized
		}
		return nil
	}
}
