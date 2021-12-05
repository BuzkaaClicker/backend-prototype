package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

type Version struct {
	Number       int    `json:"number"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Branch       string `json:"branch"`
}

type VersionController struct {
	Repo VersionRepo
}

func (c *VersionController) ServeList(w http.ResponseWriter, r *http.Request) {
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

type SqlVersionRepo struct {
	db *sql.DB
}

func (repo SqlVersionRepo) LatestVersions(ctx context.Context) ([]Version, error) {
	rows, err := repo.db.QueryContext(ctx, `
		SELECT number, os, architecture, branch from (
			select
				number,
				os,
				architecture,
				branch,
				row_number() over(partition by os, architecture, branch order by created_at desc) as row_number
			from
				version
			where
				destroyed_at is null
		) as latest where latest.row_number = 1
	`)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	versions := make([]Version, 0, 10)
	for rows.Next() {
		var version Version
		err = rows.Scan(version.Number)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		versions = append(versions, version)
	}
	return versions, nil
}
