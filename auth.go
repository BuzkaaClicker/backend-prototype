package main

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/buzkaaclicker/backend/discord"
	"github.com/gofiber/fiber/v2"
	"github.com/uptrace/bun"
)

type AuthController struct {
	DB                  *bun.DB
	OAuthUrlFactory     discord.OAuthUrlFactory
	AccessTokenExchange discord.AccessTokenExchange
	UserMeProvider      discord.UserMeProvider
}

func (c *AuthController) LoginDiscord(ctx *fiber.Ctx) error {
	code := ctx.Query("code")
	if code == "" {
		url := c.OAuthUrlFactory()
		return ctx.Redirect(url)
	} else {
		return c.authenticateDiscord(ctx, code)
	}
}

func (c *AuthController) authenticateDiscord(ctx *fiber.Ctx, code string) error {
	exchange, err := c.AccessTokenExchange(code)
	if err != nil {
		if errors.Is(err, discord.ErrOAuthInvalidCode) {
			return fiber.NewError(fiber.StatusBadRequest, "invalid code")
		} else {
			return fmt.Errorf("access token exchange: %w", err)
		}
	}

	user, err := c.UserMeProvider()(exchange.TokenType, exchange.AccessToken)
	if err != nil {
		return fmt.Errorf("discord user me: %w", err)
	}
	if user.Email == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing email")
	}

	dbCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	_, err = c.DB.NewInsert().
		Model(&User{
			DiscordId:           user.Id,
			DiscordRefreshToken: exchange.RefreshToken,
			Email:               user.Email,
			RolesNames:          []string{},
		}).
		On(`CONFLICT (discord_id) DO UPDATE SET email=EXCLUDED.email, ` +
			`discord_refresh_token=EXCLUDED.discord_refresh_token`).
		Exec(dbCtx)
	cancelFunc()
	if err != nil {
		return fmt.Errorf("user insert err: %w", err)
	}
	ctx.Status(fiber.StatusCreated).JSON("eee")
	return nil
}

func generateSessionToken() (string, error) {
	const tokenBytes = 60
	rawToken := make([]byte, tokenBytes)
	// crypto/rand - getentropy(2)
	bytesRead, err := crand.Read(rawToken)
	if err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	if bytesRead != tokenBytes {
		return "", fmt.Errorf("bytes read %d / required %d", bytesRead, tokenBytes)
	}
	return base64.StdEncoding.EncodeToString(rawToken), nil
}
