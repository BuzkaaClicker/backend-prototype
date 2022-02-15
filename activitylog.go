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

	// "beforeId" - get logs before log with given id. If lower than 0 then gets recent logs up to "limit".
	ByUserId(ctx context.Context, userId UserId, beforeId int64, limit int32) ([]ActivityLog, error)
}
