package main

import (
	"context"
	"flag"
	"log/syslog"
	"os"
	"os/signal"
	"reflect"
	"time"

	"database/sql"

	"github.com/buzkaaclicker/backend/discord"
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

func openDb(ctx context.Context, pgDsn string) *bun.DB {
	sqldb, err := sql.Open("pg", pgDsn)
	if err != nil {
		logrus.WithError(err).Fatalln("Database open failed.")
	}
	err = sqldb.Ping()
	if err != nil {
		logrus.WithError(err).Fatalln("Could not ping database.")
	}
	db := bun.NewDB(sqldb, pgdialect.New())

	models := []interface{}{
		(*User)(nil),
		(*Program)(nil),
	}
	for _, model := range models {
		modelType := reflect.TypeOf(model)
		logrus.WithField("model", modelType).Debugln("Creating table.")
		_, err = db.NewCreateTable().IfNotExists().Model(model).Exec(ctx)
		if err != nil {
			logrus.WithField("model", modelType).WithError(err).Fatalln("Could not create table.")
		}
	}
	return db
}

func logHandler() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		requestLog(ctx).Infoln("Handling request.")
		return ctx.Next()
	}
}

func authController(db *bun.DB) AuthController {
	discordClientId := os.Getenv("DISCORD_CLIENT_ID")
	if discordClientId == "" {
		logrus.Fatalln("DISCORD_CLIENT_ID not set!")
	}
	discordClientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	if discordClientId == "" {
		logrus.Fatalln("DISCORD_CLIENT_SECRET not set!")
	}
	discordRedirectUri := os.Getenv("DISCORD_AUTH_URI")
	if discordRedirectUri == "" {
		logrus.Fatalln("DISCORD_AUTH_URI not set!")
	}
	
	return AuthController{
		DB: db,
		OAuthUrlFactory: discord.RestOAuthUrlFactory(discordClientId, discordRedirectUri),
		AccessTokenExchange: discord.RestAccessTokenExchanger(discordClientId, discordClientSecret, discordRedirectUri),
		UserMeProvider: discord.RestUserMeProvider,
	}
}

func createApp(ctx context.Context, db *bun.DB) *fiber.App {
	app := fiber.New(fiber.Config{
		ReadTimeout:      5 * time.Second,
		WriteTimeout:     5 * time.Second,
		ErrorHandler:     restErrorHandler,
	})
	app.Server().MaxConnsPerIP = 4

	app.Use(logHandler())
	app.Get("/status", monitor.New())

	authController := authController(db)
	app.Get("/auth/discord", authController.LoginDiscord)

	programRepo := &PgProgramRepo{DB: db}
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

	pgDsn := os.Getenv("POSTGRES_DSN")
	if pgDsn == "" {
		logrus.Fatalln("Environment variable POSTGRES_DSN is not set!")
	}

	logrus.Infoln("Opening database.")
	db := openDb(context.Background(), pgDsn)
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
	logrus.Exit(0)
}
