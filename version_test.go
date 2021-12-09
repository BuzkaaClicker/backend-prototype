package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"flag"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

var db *bun.DB

// Start postgres docker container and initialize `db` field.
// Returns shutdown func.
func setupDb() func() {
	psgPassB := make([]byte, 30)
	if _, err := rand.Read(psgPassB); err != nil {
		logrus.WithError(err).Fatalln("Could not generate postgresql password.")
		return nil
	}
	psgPass := base32.StdEncoding.EncodeToString(psgPassB)

	pool, err := dockertest.NewPool("")
	if err != nil {
		logrus.WithError(err).Fatalln("Could not connet to docker.")
		return nil
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "14.1",
		Env:        []string{"POSTGRES_PASSWORD=" + psgPass},
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		logrus.WithError(err).Fatalln("Could not start resource.")
		return nil
	}
	resource.Expire(60)
	shutdownResource := func() {
		if err = pool.Purge(resource); err != nil {
			logrus.WithError(err).Warningln("Could not purge resource.")
		}
	}

	err = pool.Retry(func() error {
		pgDsn := fmt.Sprintf("postgresql://postgres:%s@localhost:%s/postgres?sslmode=disable",
			psgPass, resource.GetPort("5432/tcp"))
		sqldb, err := sql.Open("pg", pgDsn)
		if err != nil {
			return fmt.Errorf("sql open: %w", err)
		}

		if err = sqldb.Ping(); err != nil {
			return fmt.Errorf("sqldb ping: %w", sqldb.Ping())
		}
		db = bun.NewDB(sqldb, pgdialect.New())
		return nil
	})
	if err != nil {
		shutdownResource()
		logrus.WithError(err).Fatalln("Could not connect to database.")
		return shutdownResource
	}

	return shutdownResource
}

func TestMain(m *testing.M) {
	flag.Parse()

	var shutdownDb func()
	if !testing.Short() {
		logrus.Infoln("Starting db")
		shutdownDb = setupDb()
		if shutdownDb != nil {
			defer shutdownDb()
		}
	}

	m.Run()
}

func TestVersionLatest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	_, err := db.NewCreateTable().Model((*Version)(nil)).Exec(ctx)
	if err != nil {
		t.Fatalf("Could not create table: %s\n", err)
		return
	}
	_, err = db.NewInsert().Model(&[]Version{
		{Number: 1, OS: "macOS", Architecture: "amd64", Branch: "stable"},
		{Number: 2, OS: "macOS", Architecture: "amd64", Branch: "stable"},
		{Number: 1, OS: "Windows", Architecture: "amd64", Branch: "stable"},
		{Number: 2, OS: "Windows", Architecture: "amd64", Branch: "stable"},
		{Number: 3, OS: "Windows", Architecture: "amd64", Branch: "beta"},
		{Number: 3, OS: "Windows", Architecture: "arm64", Branch: "beta"},
	}).Exec(ctx)
	if err != nil {
		t.Fatalf("Could not insert mock versions: %s\n", err)
		return
	}

	repo := PgVersionRepo{DB: db}
	controller := VersionController{Repo: repo}

	req := httptest.NewRequest("GET", "/version/latest", nil)
	resp := httptest.NewRecorder()
	controller.ServeLatestVersions(resp, req)

	assert.Equal(resp.Header().Get("Content-Type"), contentTypeJson, "Invalid content type")
	const validResponseBody = `[{"id":2,"number":2,"os":"macOS","architecture":"amd64","branch":"stable"},` +
		`{"id":5,"number":3,"os":"Windows","architecture":"amd64","branch":"beta"},` +
		`{"id":4,"number":2,"os":"Windows","architecture":"amd64","branch":"stable"},` +
		`{"id":6,"number":3,"os":"Windows","architecture":"arm64","branch":"beta"}]` + "\n"
	assert.Equal(validResponseBody, resp.Body.String(), "Invalid response body")
}
