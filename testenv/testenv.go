package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"time"

	"github.com/buzkaaclicker/backend/pgdb"
	"github.com/buzkaaclicker/backend/profile"
	"github.com/buzkaaclicker/backend/program"
	"github.com/buzkaaclicker/backend/user"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	_ "github.com/uptrace/bun/driver/pgdriver"
)

// inspiration: https://stackoverflow.com/a/64222654 (by brpaz)

func main() {
	flag.Parse()

	logrus.Println("Starting postgres db container")
	shutdownPgDb, err := createTestPgDb()
	if err != nil {
		logrus.WithError(err).Fatalln("Could not create test database.")
	}

	var path string
	if len(os.Args) > 2 {
		path = "./../" + os.Args[2]
	} else {
		path = "./.."
	}
	logrus.WithField("path", path).Println("Running tests...")
	runTests(path)

	logrus.Println("Tests done. Shutting down test db.")
	shutdownPgDb()
}

func runTests(path string) {
	c := exec.Command("go", "test", path)
	c.Env = os.Environ()
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		logrus.WithError(err).Errorln("Could not run test command")
		return
	}
	if err := c.Wait(); err != nil {
		logrus.WithError(err).Errorln("Could not wait on test command")
		return
	}
}

// Start postgres docker container and initialize `db` field.
// Returns bun db and shutdown func OR error.
func createTestPgDb() (func(), error) {
	psgPassB := make([]byte, 30)
	if _, err := rand.Read(psgPassB); err != nil {
		return nil, fmt.Errorf("password generate: %w", err)
	}
	psgPass := base32.StdEncoding.EncodeToString(psgPassB)

	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("docker connect: %w", err)
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
		return nil, fmt.Errorf("resource start: %w", err)
	}
	resource.Expire(60)
	shutdownResource := func() {
		if err := pool.Purge(resource); err != nil {
			logrus.WithError(err).Warningln("Could not purge resource.")
		}
	}

	var pgDsn string
	pool.MaxWait = 10 * time.Second
	err = pool.Retry(func() error {
		pgDsn = fmt.Sprintf("postgresql://postgres:%s@localhost:%s/postgres?sslmode=disable",
			psgPass, resource.GetPort("5432/tcp"))
		sqldb, err := sql.Open("pg", pgDsn)
		if err != nil {
			return fmt.Errorf("sql open: %w", err)
		}

		if err = sqldb.Ping(); err != nil {
			return fmt.Errorf("sqldb ping: %w", sqldb.Ping())
		}
		bdb := bun.NewDB(sqldb, pgdialect.New())
		createDbSchema(context.Background(), bdb)
		_ = bdb.Close()
		_ = sqldb.Close()
		return nil
	})
	if err != nil {
		shutdownResource()
		return nil, fmt.Errorf("database connect: %w", err)
	}

	pgdb.SetTestEnvDsn(pgDsn)
	return shutdownResource, nil
}

func createDbSchema(ctx context.Context, db *bun.DB) {
	models := []interface{}{
		(*user.Model)(nil),
		(*profile.Model)(nil),
		(*program.Model)(nil),
	}
	for _, model := range models {
		modelType := reflect.TypeOf(model)
		logrus.WithField("model", modelType).Debugln("Creating table.")
		_, err := db.NewCreateTable().IfNotExists().Model(model).Exec(ctx)
		if err != nil {
			logrus.WithField("model", modelType).WithError(err).Fatalln("Could not create table.")
		}
	}
}
