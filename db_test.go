package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/extra/bundebug"
)

var db *bun.DB
var bdb *buntdb.DB

func TestMain(m *testing.M) {
	flag.Parse()

	var err error
	bdb, err = buntdb.Open(":memory:")
	if err != nil {
		panic(err)
	}

	var shutdownDb func()
	if !testing.Short() {
		logrus.Infoln("Starting db")
		var err error
		db, shutdownDb, err = createTestDb()
		if err != nil {
			logrus.WithError(err).Fatalln("Could not create test database.")
			return
		}
		defer shutdownDb()
	}

	m.Run()
}

// Start postgres docker container and initialize `db` field.
// Returns bun db and shutdown func OR error.
func createTestDb() (*bun.DB, func(), error) {
	psgPassB := make([]byte, 30)
	if _, err := rand.Read(psgPassB); err != nil {
		return nil, nil, fmt.Errorf("password generate: %w", err)
	}
	psgPass := base32.StdEncoding.EncodeToString(psgPassB)

	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("docker connect: %w", err)
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
		return nil, nil, fmt.Errorf("resource start: %w", err)
	}
	resource.Expire(60)
	shutdownResource := func() {
		if err = pool.Purge(resource); err != nil {
			logrus.WithError(err).Warningln("Could not purge resource.")
		}
	}

	var db *bun.DB
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
		return nil, nil, fmt.Errorf("database connect: %w", err)
	}

	if os.Getenv("DB_VERBOSE") == "true" {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}
	return db, shutdownResource, nil
}
