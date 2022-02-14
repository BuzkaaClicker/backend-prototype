package rest

import (
	"errors"
	"fmt"

	"github.com/buzkaaclicker/buzza"
	"github.com/gofiber/fiber/v2"
)

type ProgramController struct {
	Store buzza.ProgramStore
}

func (c *ProgramController) InstallTo(app *fiber.App) {
	app.Get("/download/:file_type", c.download)
}

// type, arch, os, branch
func (c *ProgramController) download(ctx *fiber.Ctx) error {
	fileType := ctx.Params("file_type", "installer")
	os := ctx.Query("os")
	arch := ctx.Query("arch")
	branch := ctx.Query("branch", "stable")

	files, err := c.Store.LatestProgramFiles(ctx.Context(), fileType, os, arch, branch)
	if err != nil {
		if errors.Is(err, buzza.ErrProgramNotFound) {
			return fiber.ErrNotFound
		} else {
			return fmt.Errorf("repo lastest program files: %w", err)
		}
	}

	type File struct {
		Path        string `json:"path"`
		DownloadUrl string `json:"downloadUrl"`
		Hash        string `json:"hash"`
	}
	mapped := make([]File, len(files))
	for i, of := range files {
		mapped[i] = File{Path: of.Path, DownloadUrl: of.DownloadUrl, Hash: of.Hash}
	}

	err = ctx.JSON(mapped)
	if err != nil {
		return fmt.Errorf("json serialize: %w", err)
	}
	return nil
}
