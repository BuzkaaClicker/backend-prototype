package rest

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/buzkaaclicker/buzza"
	"github.com/gofiber/fiber/v2"
	"github.com/tidwall/buntdb"
)

const sessionLocalsKey = "session"

type SessionController struct {
	Store buzza.SessionStore
}

func (c *SessionController) InstallTo(requestAuthorizer fiber.Handler, app *fiber.App) {
	app.Get("/session", combineHandlers(requestAuthorizer, c.serveCurrentSession))
	app.Delete("/session/:session_id", combineHandlers(requestAuthorizer, c.serveDeleteSession))
	app.Get("/sessions", combineHandlers(requestAuthorizer, c.serveSessions))
	app.Delete("/sessions/other", combineHandlers(requestAuthorizer, c.serveDeleteOtherSessions))
}

func (c *SessionController) serveCurrentSession(ctx *fiber.Ctx) error {
	session, ok := ctx.Locals(sessionLocalsKey).(buzza.Session)
	if !ok {
		return fiber.ErrUnauthorized
	}
	return ctx.JSON(session)
}

func (c *SessionController) serveSessions(ctx *fiber.Ctx) error {
	session, ok := ctx.Locals(sessionLocalsKey).(buzza.Session)
	if !ok {
		return fiber.ErrUnauthorized
	}

	activeSessions, err := c.Store.ActiveSessions(session.Token)
	if err != nil {
		if errors.Is(err, buzza.ErrSessionNotFound) {
			return fiber.ErrForbidden
		} else {
			return err
		}
	}

	// return information about session without providing access
	// to the authorization token.
	type SessionMeta struct {
		Id             string `json:"id"`
		Ip             string `json:"ip"`
		UserAgent      string `json:"userAgent"`
		LastAccessedAt int64  `json:"lastAccessedAt"`
	}
	publicInfos := make([]SessionMeta, len(activeSessions))
	for i, session := range activeSessions {
		publicInfos[i] = SessionMeta{
			Id:             session.Id,
			Ip:             session.Ip,
			UserAgent:      session.UserAgent,
			LastAccessedAt: session.LastAccessedAt.Unix(),
		}
	}
	return ctx.JSON(publicInfos)
}

func (c *SessionController) serveDeleteSession(ctx *fiber.Ctx) error {
	encodedSessionId := ctx.Params("session_id")
	if encodedSessionId == "" {
		return fiber.NewError(fiber.StatusBadRequest, "no session id")
	}
	session, ok := ctx.Locals(sessionLocalsKey).(buzza.Session)
	if !ok {
		return fiber.ErrUnauthorized
	}

	decodedSessionId, err := url.PathUnescape(encodedSessionId)
	if err != nil {
		return fmt.Errorf("unescape session id: %s", err)
	}

	if session.Id == decodedSessionId {
		err = c.Store.InvalidateByAuthToken(session.Token)
	} else {
		err = c.Store.InvalidateById(session.UserId, decodedSessionId)
	}
	if err != nil {
		if errors.Is(err, buntdb.ErrNotFound) {
			return fiber.ErrForbidden
		} else {
			return fmt.Errorf("session invalidate: %s", err)
		}
	}
	return nil
}

func (c *SessionController) serveDeleteOtherSessions(ctx *fiber.Ctx) error {
	session, ok := ctx.Locals(sessionLocalsKey).(buzza.Session)
	if !ok {
		return fiber.ErrUnauthorized
	}
	return c.Store.InvalidateAllExpect(session.Token)
}
func RequestAuthorizer(sessionStore buzza.SessionStore, userStore buzza.UserStore) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		auth := ctx.Get(fiber.HeaderAuthorization)
		if auth == "" {
			return fiber.ErrUnauthorized
		}
		if !strings.HasPrefix(auth, "Bearer ") {
			return fiber.NewError(fiber.ErrBadRequest.Code, "invalid auth type")
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		session, err := sessionStore.AcquireAndRefresh(ctx.Context(), token, ctx.IP(),
			string(ctx.Request().Header.UserAgent()))
		if err != nil {
			if errors.Is(err, buzza.ErrSessionNotFound) {
				return fiber.ErrUnauthorized
			} else {
				return fmt.Errorf("acquire and refresh session: %s", err)
			}
		}
		user, err := userStore.ById(ctx.Context(), session.UserId)
		if err != nil {
			return fmt.Errorf("retrieve user by id: %s", err)
		}

		requestLog(ctx).
			WithField("user_id", user.Id).
			Infoln("Authorized access.")

		ctx.Locals(sessionLocalsKey, session)
		ctx.Locals(userLocalsKey, user)
		return nil
	}
}
