package buzza

import (
	"context"
	"errors"
	"time"

	"github.com/buzkaaclicker/buzza/discord"
)

var ErrUserNotFound = errors.New("user not found")

type UserId int64

type Email string

type User struct {
	Id        UserId
	CreatedAt time.Time
	Roles     Roles
	Discord   UserDiscord
	Email     Email
}

// Represents info about linked discord account to our account.
type UserDiscord struct {
	Id           string
	RefreshToken string
}

type UserStore interface {
	RegisterDiscordUser(ctx context.Context, u discord.User, refreshToken string) (User, error)

	ById(ctx context.Context, userId UserId) (User, error)

	Update(ctx context.Context, user User) error
}
