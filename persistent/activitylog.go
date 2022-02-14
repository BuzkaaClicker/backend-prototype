package persistent

import (
	"context"
	"fmt"
	"time"

	"github.com/buzkaaclicker/buzza"
	"github.com/uptrace/bun"
)

type ActivityLog struct {
	bun.BaseModel `bun:"table:activity_log"`

	Id        int64                  `bun:",pk,autoincrement"`
	CreatedAt time.Time              `bun:",nullzero,notnull,default:current_timestamp"`
	UserId    int64                  `bun:",notnull"`
	Name      string                 `bun:",notnull"`
	Data      map[string]interface{} `bun:",notnull"`
}

func (l *ActivityLog) ToDomain() buzza.ActivityLog {
	return buzza.ActivityLog{
		Id:        l.Id,
		CreatedAt: l.CreatedAt,
		UserId:    buzza.UserId(l.UserId),
		Name:      l.Name,
		Data:      l.Data,
	}
}

type ActivityStore struct {
	DB *bun.DB
}

var _ buzza.ActivityStore = (*ActivityStore)(nil)

func (s *ActivityStore) AddLog(ctx context.Context, userId buzza.UserId, activity buzza.Activity) error {
	_, err := s.DB.NewInsert().
		Model(&ActivityLog{
			UserId: int64(userId),
			Name:   activity.Name,
			Data:   activity.Data,
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert log entry: %w", err)
	}
	return nil
}

func (s *ActivityStore) ByUserId(ctx context.Context, userId buzza.UserId) ([]buzza.ActivityLog, error) {
	var logs []ActivityLog
	err := s.DB.NewSelect().
		Model((*ActivityLog)(nil)).
		Where("activity_log.user_id=?", userId).
		Scan(ctx, &logs)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	ml := make([]buzza.ActivityLog, len(logs))
	for i, l := range logs {
		ml[i] = l.ToDomain()
	}
	return ml, nil
}
