package buzza

import (
	"context"
	"errors"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")

type Session struct {
	Id             string
	UserId         UserId
	Token          string
	Ip             string
	UserAgent      string
	LastAccessedAt time.Time
	ExpiresAt      time.Time
}

type SessionStore interface {
	RegisterNew(ctx context.Context, userId UserId, ip string, userAgent string) (Session, error)

	ByToken(token string) (Session, error)

	Exists(token string) (bool, error)

	ActiveSessions(token string) ([]Session, error)

	AcquireAndRefresh(ctx context.Context, token string, ip string, userAgent string) (Session, error)

	InvalidateById(userId UserId, sessionId string) error

	InvalidateByAuthToken(authToken string) error

	InvalidateAllExpect(expectToken string) error
}
