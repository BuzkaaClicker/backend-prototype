package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/uptrace/bun"
)

type Model struct {
	bun.BaseModel `bun:"table:profile"`

	Id        int64  `json:"-" bun:",pk,autoincrement"`
	UserId    int64  `json:"-" bun:",unique,notnull"`
	Name      string `json:"name" bun:",notnull"`
	AvatarUrl string `json:"avatarUrl"`
}

type Store struct {
	DB *bun.DB
}

func (s *Store) ByUserId(ctx context.Context, userId int64) (*Model, error) {
	profile := new(Model)
	err := s.DB.NewSelect().
		Model(profile).
		Where(`user_id=?`, userId).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select profile: %w", err)
	}
	return profile, nil
}

type Controller struct {
	Store Store
}

func (c *Controller) InstallTo(app *fiber.App) {
	app.Get("/profile/:user_id", c.serveProfile)
}

func (c *Controller) serveProfile(ctx *fiber.Ctx) error {
	userIdStr := ctx.Params("user_id")
	if userIdStr == "" {
		return fiber.NewError(fiber.StatusBadRequest, "no user id")
	}
	userId, err := strconv.ParseInt(userIdStr, 10, 0)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}

	profile, err := c.Store.ByUserId(ctx.Context(), int64(userId))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "profile not found")
		} else {
			return fmt.Errorf("get profile by user id: %w", err)
		}
	}
	return ctx.JSON(profile)
}
