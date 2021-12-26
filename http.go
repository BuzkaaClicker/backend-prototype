package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type ErrorResponse struct {
	ErrorMessage string `json:"error_message"`
}

func requestLog(ctx *fiber.Ctx) *logrus.Entry {
	return logrus.
		WithField("remote_addr", ctx.Context().RemoteAddr()).
		WithField("path", ctx.Path()).
		WithField("z_referer", string(ctx.Request().Header.Peek("Referer"))).
		WithField("z_user_agent", string(ctx.Request().Header.Peek("User-Agent"))).
		WithField("z_x_forwared_for", string(ctx.Request().Header.Peek("X-Forwarded-For")))
}

func restErrorHandler(ctx *fiber.Ctx, err error) error {
	if fe, ok := err.(*fiber.Error); ok {
		return ctx.
			Status(fe.Code).
			JSON(&ErrorResponse{ErrorMessage: fe.Message})
	} else {
		requestLog(ctx).WithError(err).Errorln("Internal server error.")
		// keep internal server errors private. reply with generic error message.
		return ctx.
			Status(fiber.ErrInternalServerError.Code).
			JSON(&ErrorResponse{ErrorMessage: fiber.ErrInternalServerError.Message})
	}
}

func notFoundHandler(c *fiber.Ctx) error {
	return fiber.NewError(fiber.StatusNotFound)
}
