package persistent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/buzkaaclicker/buzza"
	"github.com/uptrace/bun"
)

var ErrProgramNotFound = errors.New("program not found")

// Db model representing single program file e.g. installer, config.yml, buzkaaclickeragent.dll.
type ProgramFile struct {
	// Relative file path in BuzkaaClicker directory.
	Path string `json:"path"`
	// Download url.
	DownloadUrl string `json:"download_url"`
	// File sha256 hash.
	Hash string `json:"hash"`
}

func (f ProgramFile) ToDomain() buzza.ProgramFile {
	return buzza.ProgramFile{
		Path:        f.Path,
		DownloadUrl: f.DownloadUrl,
		Hash:        f.Hash,
	}
}

type Program struct {
	bun.BaseModel `bun:"table:program"`

	Id          int           `bun:",pk,autoincrement"`
	CreatedAt   time.Time     `bun:",nullzero,notnull,default:current_timestamp"`
	DestroyedAt sql.NullTime  `bun:",nullzero,soft_delete"`
	Type        string        `bun:",notnull,unique:build_type,type:varchar(30)"`
	OS          string        `bun:",notnull,unique:build_type,type:varchar(30)"`
	Arch        string        `bun:",notnull,unique:build_type,type:varchar(10)"`
	Branch      string        `bun:",notnull,unique:build_type,type:varchar(255)"`
	Files       []ProgramFile
}

type ProgramStore struct {
	DB *bun.DB
}

func (s ProgramStore) LatestProgramFiles(ctx context.Context, fileType string,
	os string, arch string, branch string) ([]buzza.ProgramFile, error) {
	subq := s.DB.NewSelect().
		ColumnExpr("*").
		ColumnExpr("row_number() over(partition by type, os, arch, branch order by id desc) as _row_number").
		Table("program").
		Where("type=?", fileType).
		Where("os=?", os).
		Where("arch=?", arch).
		Where("branch=?", branch)

	var files [][]ProgramFile
	err := s.DB.NewSelect().
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
		return nil, buzza.ErrProgramNotFound
	case 1:
		df := make([]buzza.ProgramFile, len(files))
		for i, f := range files[0] {
			df[i] = f.ToDomain()
		}
		return df, nil
	default:
		return nil, fmt.Errorf("too many results (%d)", filesLen)
	}
}
