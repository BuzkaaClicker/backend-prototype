package buzza

import (
	"context"
)

type Profile struct {
	Id        int64
	User      User
	Name      string
	AvatarUrl string
}

type ProfileStore interface {
	ByUserId(ctx context.Context, userId UserId) (Profile, error)
}
