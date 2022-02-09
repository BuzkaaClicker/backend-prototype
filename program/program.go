package program

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/uptrace/bun"
	_ "github.com/uptrace/bun/driver/pgdriver"
)

var ErrProgramNotFound = errors.New("program not found")

// Single program file e.g. installer, config.yml, buzkaaclickeragent.dll.
type File struct {
	// Relative file path in BuzkaaClicker directory.
	Path string `json:"path"`
	// Download url.
	DownloadUrl string `json:"download_url"`
	// File sha256 hash.
	Hash string `json:"hash"`
}

// Model model representing database entity and rest json DTO.
type Model struct {
	bun.BaseModel `bun:"table:program" json:"-"`

	Id          int           `bun:",pk,autoincrement"                            json:"id"`
	CreatedAt   time.Time     `bun:",nullzero,notnull,default:current_timestamp"  json:"-"`
	DestroyedAt sql.NullTime  `bun:",nullzero,soft_delete"                        json:"-"`
	Type        string        `bun:",notnull,unique:build_type,type:varchar(30)"  json:"type"`
	OS          string        `bun:",notnull,unique:build_type,type:varchar(30)"  json:"os"`
	Arch        string        `bun:",notnull,unique:build_type,type:varchar(10)"  json:"arch"`
	Branch      string        `bun:",notnull,unique:build_type,type:varchar(255)" json:"branch"`
	Files       []File `bun:""                                             json:"files"`
}

type Controller struct {
	Repo Repo
}

func (c *Controller) InstallTo(app *fiber.App) {
	app.Get("/download/:file_type", c.download)
}

// type, arch, os, branch
func (c *Controller) download(ctx *fiber.Ctx) error {
	fileType := ctx.Params("file_type", "installer")
	os := ctx.Query("os")
	arch := ctx.Query("arch")
	branch := ctx.Query("branch", "stable")

	files, err := c.Repo.LatestProgramFiles(ctx.Context(), fileType, os, arch, branch)
	if err != nil {
		if errors.Is(err, ErrProgramNotFound) {
			return fiber.ErrNotFound
		} else {
			return fmt.Errorf("repo lastest program files: %w", err)
		}
	}

	err = ctx.JSON(files)
	if err != nil {
		return fmt.Errorf("json serialize: %w", err)
	}
	return nil
}

type Repo interface {
	// Get latest program files matching specified arguments.
	LatestProgramFiles(ctx context.Context, fileType string,
		os string, arch string, branch string) ([]File, error)
}

type PgRepo struct {
	DB *bun.DB
}

func (repo PgRepo) LatestProgramFiles(ctx context.Context, fileType string,
	os string, arch string, branch string) ([]File, error) {
	subq := repo.DB.NewSelect().
		ColumnExpr("*").
		ColumnExpr("row_number() over(partition by type, os, arch, branch order by id desc) as _row_number").
		Table("program").
		Where("type=?", fileType).
		Where("os=?", os).
		Where("arch=?", arch).
		Where("branch=?", branch)

	var files [][]File
	err := repo.DB.NewSelect().
		TableExpr("(?) as t", subq).
		Where("t._row_number = 1").
		ColumnExpr("files").
		Scan(ctx, &files)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	filesLen := len(files)
	switch filesLen {
	case 0:
		return nil, ErrProgramNotFound
	case 1:
		return files[0], nil
	default:
		return nil, fmt.Errorf("too many results (%d)", filesLen)
	}
}
