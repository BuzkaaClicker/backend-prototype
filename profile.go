package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/uptrace/bun"
)

type Profile struct {
	bun.BaseModel `bun:"table:profile"`

	Id        int64  `json:"-" bun:",pk,autoincrement"`
	UserId    int64  `json:"-" bun:",unique,notnull"`
	Name      string `json:"name" bun:",notnull"`
	AvatarUrl string `json:"avatarUrl"`
}

type ProfileStore struct {
	DB *bun.DB
}

func (s *ProfileStore) ByUserId(ctx context.Context, userId int64) (*Profile, error) {
	profile := new(Profile)
	err := s.DB.NewSelect().
		Model(profile).
		Where(`user_id=?`, userId).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select profile: %w", err)
	}
	return profile, nil
}

type ProfileController struct {
	ProfileStore ProfileStore
}

func (c *ProfileController) ServeProfile(ctx *fiber.Ctx) error {
	userIdStr := ctx.Query("user_id")
	if userIdStr == "" {
		return fiber.NewError(fiber.StatusBadRequest, "no user id")
	}
	userId, err := strconv.ParseInt(userIdStr, 10, 0)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}

	profile, err := c.ProfileStore.ByUserId(ctx.Context(), int64(userId))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "profile not found")
		} else {
			return fmt.Errorf("get profile by user id: %w", err)
		}
	}
	return ctx.JSON(profile)
}
