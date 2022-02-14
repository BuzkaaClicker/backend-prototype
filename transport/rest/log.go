package rest

import "github.com/gofiber/fiber/v2"

func LogHandler() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		requestLog(ctx).Infoln("Handling request.")
		return ctx.Next()
	}
}

