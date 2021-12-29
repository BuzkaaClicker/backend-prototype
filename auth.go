package main

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/buzkaaclicker/backend/discord"
	"github.com/gofiber/fiber/v2"
	"github.com/tidwall/buntdb"
	"github.com/uptrace/bun"
)

const sessionTTL = 60 * 24 * time.Hour // 60 days

const SessionKey = "session"

type Session struct {
	UserId int64
	Token  string
}

type SessionStore struct {
	Buntdb *buntdb.DB
}

func (s *SessionStore) RegisterNew(userId int64) (*Session, error) {
	token, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	session := &Session{
		UserId: userId,
		Token:  token,
	}
	err = s.Buntdb.Update(func(tx *buntdb.Tx) error {
		options := &buntdb.SetOptions{
			Expires: true,
			TTL:     sessionTTL,
		}
		_, _, err := tx.Set("session:"+token, strconv.FormatInt(userId, 10), options)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("bunt update: %w", err)
	}
	return session, nil
}

func (s *SessionStore) Authorize(child fiber.Handler) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		auth := ctx.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(auth, "Bearer ") {
			return fiber.NewError(fiber.ErrBadRequest.Code, "invalid auth type")
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		var userIdRaw string
		err := s.Buntdb.View(func(tx *buntdb.Tx) error {
			var err error
			userIdRaw, err = tx.Get("session:" + token)
			return err
		})
		if err != nil {
			if err == buntdb.ErrNotFound {
				return fiber.ErrUnauthorized
			} else {
				return fmt.Errorf("could not get session: %w", err)
			}
		}
		userId, err := strconv.ParseInt(userIdRaw, 10, 0)
		if err != nil {
			return fmt.Errorf("user id raw parse: %w", err)
		}
		session := &Session{
			UserId: userId,
			Token:  token,
		}

		ctx.Locals("session", session)
		requestLog(ctx).
			WithField("user_id", userId).
			Infoln("Authorized access.")
		return child(ctx)
	}
}

type AuthController struct {
	DB                  *bun.DB
	OAuthUrlFactory     discord.OAuthUrlFactory
	AccessTokenExchange discord.AccessTokenExchange
	UserMeProvider      discord.UserMeProvider
	SessionStore        *SessionStore
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

	dcUser, err := c.UserMeProvider()(exchange.TokenType, exchange.AccessToken)
	if err != nil {
		return fmt.Errorf("discord user me: %w", err)
	}
	if dcUser.Email == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing email")
	}

	dbCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	user := &User{
		DiscordId:           dcUser.Id,
		DiscordRefreshToken: exchange.RefreshToken,
		Email:               dcUser.Email,
		RolesNames:          []string{},
	}
	_, err = c.DB.NewInsert().
		Model(user).
		On(`CONFLICT (discord_id) DO UPDATE SET email=EXCLUDED.email, ` +
			`discord_refresh_token=EXCLUDED.discord_refresh_token`).
		Exec(dbCtx)
	cancelFunc()
	if err != nil {
		return fmt.Errorf("user insert err: %w", err)
	}

	session, err := c.SessionStore.RegisterNew(user.Id)
	if err != nil {
		return fmt.Errorf("session register new: %w", err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(map[string]interface{}{
		"user_id":      session.UserId,
		"access_token": session.Token,
		"expires_in":   time.Now().Add(sessionTTL).Unix(),
	})
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
