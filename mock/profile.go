package mock

import (
	"context"

	"github.com/buzkaaclicker/buzza"
)

type ProfileService struct {
	ByUserIdFn func(ctx context.Context, userId buzza.UserId) (buzza.Profile, error)
}

func (s ProfileService) ByUserId(ctx context.Context, userId buzza.UserId) (buzza.Profile, error) {
	return s.ByUserIdFn(ctx, userId)
}
