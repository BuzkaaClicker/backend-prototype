package main

import (
	"context"
	"fmt"
	"time"

	"github.com/buzkaaclicker/backend/discord"
	"github.com/uptrace/bun"
)

const UserKey = "user"

type User struct {
	bun.BaseModel `bun:"table:user"`

	Id                  int64     `bun:",pk,autoincrement" json:"-"`
	CreatedAt           time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"-"`
	RolesNames          []RoleId  `bun:",notnull,array" json:"-"`
	DiscordId           string    `bun:",notnull,unique" json:"-"`
	DiscordRefreshToken string    `bun:",notnull" json:"-"`
	Email               string    `bun:"email,notnull" json:"-"`

	// Mapped (in AfterScanRow hook) roles from RolesNames.
	Roles Roles `bun:"-"`
}

var _ bun.AfterScanRowHook = (*User)(nil)

func (u *User) AfterScanRow(ctx context.Context) error {
	u.Roles = make(Roles, 0, len(u.RolesNames))
	for _, n := range u.RolesNames {
		role, ok := AllRoles[n]
		if ok {
			u.Roles = append(u.Roles, role)
		}
	}
	return nil
}

type UserStore struct {
	DB *bun.DB
}

func (s *UserStore) RegisterDiscordUser(ctx context.Context,
	dcUser discord.User, refreshToken string) (*User, error) {
	user := &User{
		DiscordId:           dcUser.Id,
		DiscordRefreshToken: refreshToken,
		Email:               dcUser.Email,
		RolesNames:          []RoleId{},
	}

	_, err := s.DB.NewInsert().
		Model(user).
		On(`CONFLICT (discord_id) DO UPDATE SET email=EXCLUDED.email, ` +
			`discord_refresh_token=EXCLUDED.discord_refresh_token`).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

func (s *UserStore) ById(ctx context.Context, userId int64) (*User, error) {
	user := new(User)
	err := s.DB.NewSelect().
		Model(user).
		Where("id=?", userId).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select user: %w", err)
	}
	return user, nil
}
