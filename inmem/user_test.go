package inmem

import (
	"context"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
	"github.com/stretchr/testify/assert"
)

func TestUserStore(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	s := NewUserStore()
	_, err := s.ById(ctx, 1)
	assert.Equal(buzza.ErrUserNotFound, err)

	const refreshToken = "ZJKdfsjklAUIdsaioj"
	u, err := s.RegisterDiscordUser(ctx, discord.User{
		Id:         "20d93290snowflake",
		Username:   "indecorum",
		Email:      "aleja@rejwu.pl",
		AvatarHash: "SLD",
	}, refreshToken)
	if !assert.NoError(err) {
		return
	}

	ufound, err := s.ById(ctx, u.Id)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(u, ufound)
}
