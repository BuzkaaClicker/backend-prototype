package main

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:user"`

	Id                  int32     `bun:",pk,autoincrement" json:"-"`
	CreatedAt           time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"-"`
	RolesNames          []string  `bun:",notnull,array" json:"-"`
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
