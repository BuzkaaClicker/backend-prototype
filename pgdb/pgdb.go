package pgdb

import (
	"context"
	"database/sql"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	_ "github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

func Open(ctx context.Context, pgDsn string) *bun.DB {
	sqldb, err := sql.Open("pg", pgDsn)
	if err != nil {
		logrus.WithError(err).Fatalln("Could not open pg database.")
	}
	err = sqldb.Ping()
	if err != nil {
		logrus.WithError(err).Fatalln("Could not ping pg database.")
	}

	bdb := bun.NewDB(sqldb, pgdialect.New())
	if os.Getenv("DB_VERBOSE") == "true" {
		// color.NoColor = false
		bdb.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}
	return bdb
}

// Running integration tests requires real pg db instance, but we
// don't have enought time to start db for every test so we will start db once
// and then pass datasource to as many tests as we want.

func OpenTest(ctx context.Context) *bun.DB {
	return Open(ctx, TestEnvDsn())
}

func TestEnvDsn() string {
	return os.Getenv("PGDB_DSN")
}

func SetTestEnvDsn(dsn string) {
	os.Setenv("PGDB_DSN", dsn)
}
