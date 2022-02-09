package user

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type ActivityLog struct {
	bun.BaseModel `bun:"table:user"`

	Id        int64                  `bun:",pk,autoincrement"`
	CreatedAt time.Time              `bun:",nullzero,notnull,default:current_timestamp"`
	UserId    int64                  `bun:",notnull"`
	Name      string                 `bun:",notnull"`
	Data      map[string]interface{} `bun:",notnull"`
}

type ActivityStore struct {
	DB *bun.DB
}

func (s *ActivityStore) ByUserId(ctx context.Context, userId int64) (*ActivityLog, error) {
	return nil, nil
}
