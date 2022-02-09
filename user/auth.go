package user

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
	"github.com/buzkaaclicker/backend/rest"
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
	Buntdb    *buntdb.DB
	UserStore *Store
}

func (s *SessionStore) RegisterNew(userId int64) (*Session, error) {
	dirtyToken, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	// replace all ":" with "_" to make our
	// session store queries at buntdb BUNTDBinjection safe
	// (for example, if later someone will add key "session:token:random_sufix" then
	// without line below theorically it can overwrite this random sufix)
	token := strings.Replace(dirtyToken, ":", "_", -1)

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

func (s *SessionStore) Exists(token string) (bool, error) {
	err := s.Buntdb.View(func(tx *buntdb.Tx) error {
		_, err := tx.Get("session:" + token)
		return err
	})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, buntdb.ErrNotFound):
		return false, nil
	default:
		return false, fmt.Errorf("bunt view: %w", err)
	}
}

func (s *SessionStore) Invalidate(token string) error {
	err := s.Buntdb.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Delete("session:" + token)
		return err
	})
	if err != nil {
		return fmt.Errorf("bunt update: %w", err)
	}
	return nil
}

func (s *SessionStore) Authorize(ctx *fiber.Ctx) error {
	auth := ctx.Get(fiber.HeaderAuthorization)
	if auth == "" {
		return fiber.ErrUnauthorized
	}
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
	user, err := s.UserStore.ById(ctx.Context(), session.UserId)
	if err != nil {
		return fmt.Errorf("retrieve user by id: %w", err)
	}

	rest.RequestLog(ctx).
		WithField("user_id", userId).
		Infoln("Authorized access.")

	ctx.Locals(SessionKey, session)
	ctx.Locals(LocalsKey, user)
	return nil
}

type AuthController struct {
	DB                    *bun.DB
	CreateDiscordOAuthUrl discord.OAuthUrlFactory
	ExchangeAccessToken   discord.AccessTokenExchanger
	UserMeProvider        discord.UserMeProvider
	GuildMemberAdd        discord.GuildMemberAdd
	SessionStore          *SessionStore
	UserStore             *Store
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
		rest.RequestLog(ctx).WithError(err).Infoln("Invalid body.")
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

	guildAddStatus, err := c.GuildMemberAdd(exchange.AccessToken, dcUser.Id); 
	if err != nil {
		if errors.Is(err, discord.ErrUnauthorized) {
			return fiber.NewError(fiber.StatusUnauthorized, "discord guild join unauthorized")
		} else {
			return err
		}
	}
	rest.RequestLog(ctx).Infof("Discord guild member add status: %d\n", guildAddStatus)

	dbCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	user, err := c.UserStore.RegisterDiscordUser(dbCtx, dcUser, exchange.RefreshToken)
	cancelFunc()
	if err != nil {
		return fmt.Errorf("user register: %w", err)
	}

	session, err := c.SessionStore.RegisterNew(user.Id)
	if err != nil {
		return fmt.Errorf("session register new: %w", err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(map[string]interface{}{
		"userId":      session.UserId,
		"accessToken": session.Token,
		"expiresAt":   time.Now().Add(sessionTTL).Unix(),
	})
}

func (c *AuthController) logoutHandler() fiber.Handler {
	return rest.CombineHandlers(c.SessionStore.Authorize, func(ctx *fiber.Ctx) error {
		session := ctx.Locals(SessionKey).(*Session)
		return c.SessionStore.Invalidate(session.Token)
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
