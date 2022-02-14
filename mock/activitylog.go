package mock

import (
	"context"

	"github.com/buzkaaclicker/buzza"
)

type ActivityStore struct {
	AddLogFn   func(ctx context.Context, userId buzza.UserId, activity buzza.Activity) error

	ByUserIdFn func(ctx context.Context, userId buzza.UserId) ([]buzza.ActivityLog, error)
}

func (s ActivityStore) AddLog(ctx context.Context, userId buzza.UserId, activity buzza.Activity) error {
	return s.AddLogFn(ctx, userId, activity)
}

func (s ActivityStore) ByUserId(ctx context.Context, userId buzza.UserId) ([]buzza.ActivityLog, error) {
	return s.ByUserIdFn(ctx, userId)
}
