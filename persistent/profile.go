package persistent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/buzkaaclicker/buzza"
	"github.com/gofiber/fiber/v2"
	"github.com/uptrace/bun"
)

type Profile struct {
	bun.BaseModel `bun:"table:profile"`

	Id        int64  `bun:",pk,autoincrement"`
	UserId    int64  `bun:",unique,notnull"`
	User      *User  `bun:"rel:belongs-to"`
	Name      string `bun:",notnull"`
	AvatarUrl string
}

func (p Profile) ToDomain() buzza.Profile {
	return buzza.Profile{
		Id:        p.Id,
		User:      p.User.ToDomain(),
		Name:      p.Name,
		AvatarUrl: p.AvatarUrl,
	}
}

type ProfileStore struct {
	DB *bun.DB
}

var _ buzza.ProfileStore = (*ProfileStore)(nil)

func (s *ProfileStore) ByUserId(ctx context.Context, userId buzza.UserId) (buzza.Profile, error) {
	profile := new(Profile)
	err := s.DB.NewSelect().
		Model(profile).
		Relation("User").
		Where(`user_id=?`, userId).
		Scan(ctx)
	if err != nil {
		return buzza.Profile{}, fmt.Errorf("select profile: %w", err)
	}
	return profile.ToDomain(), nil
}

type ProfileController struct {
	ProfileStore ProfileStore
}

func (c *ProfileController) ServeProfile(ctx *fiber.Ctx) error {
	userIdStr := ctx.Params("user_id")
	if userIdStr == "" {
		return fiber.NewError(fiber.StatusBadRequest, "no user id")
	}
	userId, err := strconv.ParseInt(userIdStr, 10, 0)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}

	profile, err := c.ProfileStore.ByUserId(ctx.Context(), buzza.UserId(userId))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "profile not found")
		} else {
			return fmt.Errorf("get profile by user id: %w", err)
		}
	}
	return ctx.JSON(profile)
}
