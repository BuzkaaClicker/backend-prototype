package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
	_ "github.com/uptrace/bun/driver/pgdriver"
)

// Version model representing database entity and rest json DTO.
type Version struct {
	bun.BaseModel `bun:"table:version"`

	Id           int          `bun:",pk,autoincrement"                           json:"id"`
	CreatedAt    time.Time    `bun:",nullzero,notnull,default:current_timestamp" json:"-"`
	DestroyedAt  sql.NullTime `bun:",nullzero,soft_delete"                       json:"-"`
	Number       int          `bun:",notnull,unique:build_type"                  json:"number"`
	OS           string       `bun:",notnull,unique:build_type"                  json:"os"`
	Architecture string       `bun:",notnull,unique:build_type"                  json:"architecture"`
	Branch       string       `bun:",notnull,unique:build_type"                  json:"branch"`
}

type VersionController struct {
	Repo VersionRepo
}

func (c *VersionController) ServeLatestVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := c.Repo.LatestVersions(r.Context())
	if err != nil {
		logrus.WithError(err).Errorln("Get latest version from repo failed!")
		writeInternalError(w, "get latest version from repo failed")
		return
	}

	setJsonContentType(w.Header())
	err = json.NewEncoder(w).Encode(versions)
	if err != nil {
		requestLog(r).WithError(err).Errorln("JSON encode/write failed")
		writeInternalError(w, "JSON encode/write failed")
		return
	}
}

type VersionRepo interface {
	// Get latest versions for every branches and every compatible platforms.
	LatestVersions(ctx context.Context) ([]Version, error)
}

type PgVersionRepo struct {
	DB *bun.DB
}

func (repo PgVersionRepo) LatestVersions(ctx context.Context) ([]Version, error) {
	subq := repo.DB.NewSelect().
		ColumnExpr("*").
		ColumnExpr("row_number() over(partition by os, architecture, branch order by id desc) as _row_number").
		Table("version")

	var versions []Version
	err := repo.DB.NewSelect().
		TableExpr("(?) as t", subq).
		Where("t._row_number = 1").
		ColumnExpr("*").
		Scan(ctx, &versions)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return versions, nil
}
