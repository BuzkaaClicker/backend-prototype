package buzza

import (
	"context"
	"time"
)

type Activity struct {
	Name string
	Data map[string]interface{}
}

type ActivityLog struct {
	Id        int64
	CreatedAt time.Time
	UserId    UserId
	Name      string
	Data      map[string]interface{}
}

type ActivityStore interface {
	AddLog(ctx context.Context, userId UserId, activity Activity) error

	ByUserId(ctx context.Context, userId UserId) ([]ActivityLog, error)
}
