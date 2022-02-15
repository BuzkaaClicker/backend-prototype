package rest

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
	"github.com/gofiber/fiber/v2"
)

type ProfileController struct {
	Store          buzza.ProfileStore
	UserMeProvider discord.UserMeProvider
}

func (c *ProfileController) InstallTo(app *fiber.App) {
	app.Get("/profile/:user_id", c.serveProfile)
}

func (c *ProfileController) serveProfile(ctx *fiber.Ctx) error {
	userIdStr := ctx.Params("user_id")
	if userIdStr == "" {
		return fiber.NewError(fiber.StatusBadRequest, "no user id")
	}
	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}

	profile, err := c.Store.ByUserId(ctx.Context(), buzza.UserId(userId))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "profile not found")
		} else {
			return fmt.Errorf("get profile by user id: %w", err)
		}
	}

	type ProfileResponse struct {
		Name      string `json:"name"`
		AvatarUrl string `json:"avatarUrl"`
	}
	return ctx.JSON(ProfileResponse{
		Name:      profile.Name,
		AvatarUrl: profile.AvatarUrl,
	})
}
