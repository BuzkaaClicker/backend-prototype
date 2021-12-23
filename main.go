package main

import (
	"context"
	"flag"
	"log/syslog"
	"os"
	"os/signal"
	"time"

	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/sirupsen/logrus"
	logrusys "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	_ "github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

func setupLogger(verbose bool) {
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: time.Stamp,
		FullTimestamp:   true,
	})
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	syslogHook, err := logrusys.NewSyslogHook("", "", syslog.LOG_USER, "clicker_backend")
	if err != nil {
		logrus.WithError(err).Fatalln("Could not create syslog hook.")
		return
	}
	logrus.AddHook(syslogHook)
}

func openDb(pgDsn string) *bun.DB {
	sqldb, err := sql.Open("pg", pgDsn)
	if err != nil {
		logrus.WithError(err).Fatalln("Database open failed.")
	}
	err = sqldb.Ping()
	if err != nil {
		logrus.WithError(err).Fatalln("Could not ping database.")
	}
	return bun.NewDB(sqldb, pgdialect.New())
}

func logHandler() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		requestLog(ctx).Infoln("Handling request.")
		return ctx.Next()
	}
}

func createApp(ctx context.Context, db *bun.DB) *fiber.App {
	app := fiber.New(fiber.Config{
		ReadTimeout:      5 * time.Second,
		WriteTimeout:     5 * time.Second,
		DisableKeepalive: true,
		ErrorHandler:     restErrorHandler,
	})
	app.Server().MaxConnsPerIP = 4

	app.Use(logHandler())
	app.Get("/status", monitor.New())

	programRepo := &PgProgramRepo{DB: db}
	if err := programRepo.PrepareDb(ctx); err != nil {
		logrus.WithError(err).Fatalln("Could not prepare program repo db.")
	}

	programController := ProgramController{Repo: programRepo}
	app.Get("/download/:file_type", programController.Download)

	app.Use(notFoundHandler)
	return app
}

func awaitInterruption() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func main() {
	flag.Parse()
	verbose := os.Getenv("VERBOSE") == "true"
	setupLogger(verbose)
	logrus.Infoln("Starting backend.")
	defer logrus.Exit(0)

	pgDsn := os.Getenv("POSTGRES_DSN")
	if pgDsn == "" {
		logrus.Fatalln("Environment variable POSTGRES_DSN is not set!")
	}

	logrus.Infoln("Opening database.")
	db := openDb(pgDsn)
	if verbose {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}
	defer db.DB.Close()
	defer db.Close()

	logrus.Infoln("Creating fiber app.")
	fiberApp := createApp(context.Background(), db)
	go fiberApp.Listen("127.0.0.1:2137")

	logrus.Infoln("Starting listening... To shut down use ^C")

	awaitInterruption()
	logrus.Infoln("Shutting down...")
	err := fiberApp.Shutdown()
	if err != nil {
		logrus.WithError(err).Warningln("Fiber shutdown failed.")
	}
}
