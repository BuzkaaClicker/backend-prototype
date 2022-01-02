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
	"github.com/tidwall/buntdb"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	_ "github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

type app struct {
	bdb               *buntdb.DB
	db                *bun.DB
	userStore         UserStore
	sessionStore      SessionStore
	authController    AuthController
	programRepo       ProgramRepo
	programController ProgramController
	server            *fiber.App
}

func newApp(
	ctx context.Context,
	bdb *buntdb.DB,
	db *bun.DB,
	discordConfig discordConfig,
) *app {
	createDbSchema(ctx, db)

	var app app
	app.bdb = bdb
	app.db = db
	app.userStore = UserStore{DB: db}
	app.sessionStore = SessionStore{Buntdb: bdb, UserStore: &app.userStore}

	app.authController = AuthController{
		DB:                   db,
		OAuthUrlFactory:      discordConfig.OAuthUrlFactory(),
		AccessTokenExchanger: discordConfig.AccessTokenExchanger(),
		UserMeProvider:       discord.RestUserMeProvider,
		SessionStore:         &app.sessionStore,
		UserStore:            &app.userStore,
	}

	app.programRepo = &PgProgramRepo{DB: db}
	app.programController = ProgramController{Repo: app.programRepo}

	app.server = createServer(&app.sessionStore, &app.authController, &app.programController)
	return &app
}

func (a *app) ListenAndServe() {
	a.server.Listen("127.0.0.1:2137")
}

func (a *app) Shutdown() error {
	return a.server.Shutdown()
}

func createServer(sessions *SessionStore, auth *AuthController, program *ProgramController) *fiber.App {
	app := fiber.New(fiber.Config{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		ErrorHandler: restErrorHandler,
	})
	app.Server().MaxConnsPerIP = 4
	app.Use(logHandler())

	app.Get("/status", combineHandlers(
		sessions.Authorize, RequirePermissions(PermissionAdminDashboard), monitor.New()))
	app.Get("/auth/discord", auth.LoginDiscord)
	app.Get("/download/:file_type", program.Download)

	app.Use(notFoundHandler)
	return app
}

func createDbSchema(ctx context.Context, db *bun.DB) {
	models := []interface{}{
		(*User)(nil),
		(*Program)(nil),
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
	return db
}

func logHandler() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		requestLog(ctx).Infoln("Handling request.")
		return ctx.Next()
	}
}

type discordConfig struct {
	clientId             string
	clientSecret         string
	redirectUri          string
	oauthUrlFactory      discord.OAuthUrlFactory
	accessTokenExchanger discord.AccessTokenExchanger
}

func (c discordConfig) OAuthUrlFactory() discord.OAuthUrlFactory {
	if c.oauthUrlFactory == nil {
		return discord.RestOAuthUrlFactory(c.clientId, c.redirectUri)
	} else {
		return c.oauthUrlFactory
	}
}

func (c discordConfig) AccessTokenExchanger() discord.AccessTokenExchanger {
	if c.oauthUrlFactory == nil {
		return discord.RestAccessTokenExchanger(c.clientId, c.clientSecret, c.redirectUri)
	} else {
		return c.accessTokenExchanger
	}
}

func discordConfigFromEnv() discordConfig {
	clientId := os.Getenv("DISCORD_CLIENT_ID")
	if clientId == "" {
		logrus.Fatalln("DISCORD_CLIENT_ID not set!")
	}
	clientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	if clientSecret == "" {
		logrus.Fatalln("DISCORD_CLIENT_SECRET not set!")
	}
	redirectUri := os.Getenv("DISCORD_AUTH_URI")
	if redirectUri == "" {
		logrus.Fatalln("DISCORD_AUTH_URI not set!")
	}
	return discordConfig{clientId, clientSecret, redirectUri, nil, nil}
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

	bdb, err := buntdb.Open("kv.db")
	if err != nil {
		logrus.WithError(err).Fatalln("Could not open buntdb.")
	}
	defer bdb.Close()

	logrus.Infoln("Opening database.")
	db := openDb(context.Background(), pgDsn)
	if verbose {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}
	defer db.DB.Close()
	defer db.Close()

	discordConfig := discordConfigFromEnv()

	logrus.Infoln("Creating app.")
	app := newApp(context.Background(), bdb, db, discordConfig)
	go app.ListenAndServe()

	logrus.Infoln("Starting listening... To shut down use ^C")
	awaitInterruption()

	logrus.Infoln("Shutting down...")
	err = app.Shutdown()
	if err != nil {
		logrus.WithError(err).Warningln("Fiber shutdown failed.")
	}
	logrus.Exit(0)
}
