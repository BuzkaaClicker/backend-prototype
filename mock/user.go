package mock

import (
	"context"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
)

type UserStore struct {
	RegisterDiscordUserFn func(ctx context.Context, u discord.User, refreshToken string) (buzza.User, error)

	ByIdFn func(ctx context.Context, userId buzza.UserId) (buzza.User, error)

	UpdateFn func(ctx context.Context, user buzza.User) error
}

func (s UserStore) RegisterDiscordUser(ctx context.Context, u discord.User, refreshToken string) (buzza.User, error) {
	return s.RegisterDiscordUserFn(ctx, u, refreshToken)
}

func (s UserStore) ById(ctx context.Context, userId buzza.UserId) (buzza.User, error) {
	return s.ByIdFn(ctx, userId)
}

func (s UserStore) Update(ctx context.Context, user buzza.User) error {
	return s.UpdateFn(ctx, user)
}
