package rest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
	"github.com/gofiber/fiber/v2"
)

type AuthController struct {
	CreateDiscordOAuthUrl discord.OAuthUrlFactory
	ExchangeAccessToken   discord.AccessTokenExchanger
	UserMeProvider        discord.UserMeProvider
	GuildMemberAdd        discord.GuildMemberAdd
	SessionStore          buzza.SessionStore
	UserStore             buzza.UserStore
}

func (c *AuthController) InstallTo(app *fiber.App) {
	app.Get("/auth/discord", c.serveCreateDiscordOAuthUrl)
	app.Post("/auth/discord", c.serveAuthenticateDiscord)
	app.Post("/auth/logout", c.logoutHandler())
}

func (c *AuthController) serveCreateDiscordOAuthUrl(ctx *fiber.Ctx) error {
	url := c.CreateDiscordOAuthUrl()
	return ctx.JSON(map[string]string{
		"url": url,
	})
}

func (c *AuthController) serveAuthenticateDiscord(ctx *fiber.Ctx) error {
	body := struct {
		Code string `json:"code"`
	}{}
	if err := ctx.BodyParser(&body); err != nil {
		requestLog(ctx).WithError(err).Infoln("Invalid body.")
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	code := body.Code
	if code == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid code")
	}

	exchange, err := c.ExchangeAccessToken(code)
	if err != nil {
		if errors.Is(err, discord.ErrOAuthInvalidCode) {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid code")
		} else {
			return fmt.Errorf("access token exchange: %w", err)
		}
	}

	dcUser, err := c.UserMeProvider()(exchange.Token())
	if err != nil {
		return fmt.Errorf("discord user me: %w", err)
	}
	if dcUser.Email == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing email")
	}

	guildAddStatus, err := c.GuildMemberAdd(exchange.AccessToken, dcUser.Id)
	if err != nil {
		if errors.Is(err, discord.ErrUnauthorized) {
			return fiber.NewError(fiber.StatusUnauthorized, "discord guild join unauthorized")
		} else {
			return err
		}
	}
	requestLog(ctx).Infof("Discord guild member add status: %d\n", guildAddStatus)

	dbCtx, cancelFunc := context.WithTimeout(context.Background(), time.Minute)
	user, err := c.UserStore.RegisterDiscordUser(dbCtx, dcUser, exchange.RefreshToken)
	cancelFunc()
	if err != nil {
		return fmt.Errorf("user register: %w", err)
	}
	session, err := c.SessionStore.RegisterNew(ctx.Context(), user.Id, ctx.IP(), string(ctx.Request().Header.UserAgent()))
	if err != nil {
		return fmt.Errorf("session register new: %w", err)
	}

	return ctx.Status(fiber.StatusCreated).JSON(map[string]interface{}{
		"id":          session.Id,
		"userId":      session.UserId,
		"accessToken": session.Token,
		"expiresAt":   session.ExpiresAt.Unix(),
	})
}

func (c *AuthController) logoutHandler() fiber.Handler {
	return combineHandlers(RequestAuthorizer(c.SessionStore, c.UserStore), func(ctx *fiber.Ctx) error {
		session := ctx.Locals(sessionLocalsKey).(buzza.Session)
		return c.SessionStore.InvalidateByAuthToken(session.Token)
	})
}
