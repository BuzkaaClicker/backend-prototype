package persistent

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:user"`

	Id                  int64          `bun:",pk,autoincrement"`
	CreatedAt           time.Time      `bun:",nullzero,notnull,default:current_timestamp"`
	RolesNames          []buzza.RoleId `bun:",notnull,array"`
	DiscordId           string         `bun:",notnull,unique"`
	DiscordRefreshToken string         `bun:",notnull"`
	Email               string         `bun:"email,notnull"`
	Profile             *Profile       `bun:"rel:has-one,join:id=user_id"`

	// Mapped (in AfterScanRow hook) roles from RolesNames.
	Roles buzza.Roles `bun:"-"`
}

func (u User) ToDomain() buzza.User {
	return buzza.User{
		Id:                 buzza.UserId(u.Id),
		CreatedAt:          u.CreatedAt,
		Roles:              u.Roles,
		Discord: buzza.UserDiscord{Id: u.DiscordId, RefreshToken: u.DiscordRefreshToken},
		Email:              buzza.Email(u.Email),
	}
}

var _ bun.AfterScanRowHook = (*User)(nil)

func (u *User) AfterScanRow(ctx context.Context) error {
	u.Roles = make(buzza.Roles, 0, len(u.RolesNames))
	for _, n := range u.RolesNames {
		role, ok := buzza.AllRoles[n]
		if ok {
			u.Roles = append(u.Roles, role)
		}
	}
	return nil
}

type UserStore struct {
	DB *bun.DB
}

var _ buzza.UserStore = (*UserStore)(nil)

func (s *UserStore) RegisterDiscordUser(ctx context.Context, u discord.User, refreshToken string) (buzza.User, error) {
	user := &User{
		RolesNames:          []buzza.RoleId{},
		DiscordId:           u.Id,
		DiscordRefreshToken: refreshToken,
		Email:               u.Email,
	}

	err := s.DB.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewInsert().
			Model(user).
			On(`CONFLICT (discord_id) DO UPDATE SET email=EXCLUDED.email, ` +
				`discord_refresh_token=EXCLUDED.discord_refresh_token`).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("insert user: %w", err)
		}

		profile := &Profile{
			UserId:    user.Id,
			Name:      u.Username,
			AvatarUrl: u.AvatarUrl(),
		}
		_, err = tx.NewInsert().
			Model(profile).
			On(`CONFLICT (user_id) DO UPDATE SET name=EXCLUDED.name, avatar_url=EXCLUDED.avatar_url`).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("insert profile: %w", err)
		}
		return nil
	})
	if err != nil {
		return buzza.User{}, err
	}

	return user.ToDomain(), nil
}

func (s *UserStore) ById(ctx context.Context, userId buzza.UserId) (buzza.User, error) {
	user := new(User)
	err := s.DB.NewSelect().
		Model(user).
		Where(`"user"."id"=?`, userId).
		Relation("Profile").
		Scan(ctx)
	if err != nil {
		return buzza.User{}, fmt.Errorf("select user: %w", err)
	}
	return user.ToDomain(), nil
}

func (s *UserStore) Update(ctx context.Context, user buzza.User) error {
	_, err := s.DB.NewUpdate().
		Model(user).
		Where(`id=?`, user.Id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update query: %w", err)
	}
	return nil
}
