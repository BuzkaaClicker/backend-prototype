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
	"github.com/gofiber/fiber/v2/middleware/cors"
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
	profileStore      ProfileStore
	sessionStore      SessionStore
	authController    AuthController
	programRepo       ProgramRepo
	programController ProgramController
	profileController ProfileController
	server            *fiber.App
}

func newApp(
	ctx context.Context,
	bdb *buntdb.DB,
	db *bun.DB,
	discordConfig discordConfig,
	debug bool,
	// Called right before server registers not found handler at end of the route stack.
	// Required by tests to register their custom test routes.
	configureServer func(app *app),
) *app {
	createDbSchema(ctx, db)

	var app app
	app.bdb = bdb
	app.db = db
	app.userStore = UserStore{DB: db}
	app.profileStore = ProfileStore{DB: db}
	app.sessionStore = SessionStore{Buntdb: bdb, UserStore: &app.userStore}

	app.authController = AuthController{
		DB:                    db,
		CreateDiscordOAuthUrl: discordConfig.oauthUrlFactory,
		ExchangeAccessToken:   discordConfig.accessTokenExchanger,
		UserMeProvider:        discord.RestUserMeProvider,
		GuildMemberAdd:        discordConfig.guildMemberAdd,
		SessionStore:          &app.sessionStore,
		UserStore:             &app.userStore,
	}

	app.programRepo = &PgProgramRepo{DB: db}
	app.programController = ProgramController{Repo: app.programRepo}
	app.profileController = ProfileController{ProfileStore: app.profileStore}

	createServer(func(server *fiber.App) {
		app.server = server

		server.Use(logHandler())

		if configureServer != nil {
			configureServer(&app)
		}

		api := fiber.New(fiber.Config{
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			ErrorHandler: restErrorHandler,
		})

		allowOrigins := "https://buzkaaclicker.pl"
		if debug {
			allowOrigins += ", http://test.buzkaaclicker.pl:3000"
		}
		api.Use(cors.New(cors.Config{AllowOrigins: allowOrigins}))

		api.Get("/status", combineHandlers(
			app.sessionStore.Authorize, RequirePermissions(PermissionAdminDashboard), monitor.New()))
		api.Get("/auth/discord", app.authController.ServeCreateDiscordOAuthUrl)
		api.Post("/auth/discord", app.authController.ServeAuthenticateDiscord)
		api.Post("/auth/logout", app.authController.ServeLogout())

		api.Get("/download/:file_type", app.programController.Download)
		api.Get("/profile/:user_id", app.profileController.ServeProfile)
		server.Mount("/api/", api)

		server.Static("/", "./www/", fiber.Static{
			Browse: false,
			Index:  "index.html",
		})

		server.Use(notFoundHandler)
	})
	return &app
}

func (a *app) ListenAndServe(debug bool) {
	var addr string
	if debug {
		addr = "127.0.0.1:2137"
	} else {
		addr = ":2137"
	}
	a.server.Listen(addr)
}

func (a *app) Shutdown() error {
	return a.server.Shutdown()
}

func createServer(configure func(server *fiber.App)) *fiber.App {
	server := fiber.New(fiber.Config{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		ErrorHandler: restErrorHandler,
	})
	server.Server().MaxConnsPerIP = 20
	server.Use(logHandler())

	configure(server)

	server.Use(notFoundHandler)
	return server
}

func createDbSchema(ctx context.Context, db *bun.DB) {
	models := []interface{}{
		(*User)(nil),
		(*Profile)(nil),
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
	guildMemberAdd       discord.GuildMemberAdd
}

func discordConfigFromEnv() discordConfig {
	requireEnv := func(key string) string {
		value := os.Getenv(key)
		if value == "" {
			logrus.Fatalln(key + " not set!")
		}
		return value
	}
	clientId := requireEnv("DISCORD_CLIENT_ID")
	clientSecret := requireEnv("DISCORD_CLIENT_SECRET")
	redirectUri := requireEnv("DISCORD_AUTH_URI")
	guildId := requireEnv("DISCORD_GUILD_ID")
	botToken := requireEnv("DISCORD_BOT_TOKEN")
	return discordConfig{
		clientId,
		clientSecret,
		redirectUri,
		discord.RestOAuthUrlFactory(clientId, redirectUri),
		discord.RestAccessTokenExchanger(clientId, clientSecret, redirectUri),
		discord.RestGuildMemberAdd(botToken, guildId),
	}
}

func awaitInterruption() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func main() {
	flag.Parse()
	debug := os.Getenv("DEBUG") == "true"
	setupLogger(debug)
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
	if debug {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}
	defer db.DB.Close()
	defer db.Close()

	discordConfig := discordConfigFromEnv()

	logrus.Infoln("Creating app.")
	app := newApp(context.Background(), bdb, db, discordConfig, debug, nil)
	go app.ListenAndServe(debug)

	logrus.Infoln("Starting listening... To shut down use ^C")
	awaitInterruption()

	logrus.Infoln("Shutting down...")
	err = app.Shutdown()
	if err != nil {
		logrus.WithError(err).Warningln("Fiber shutdown failed.")
	}
	logrus.Exit(0)
}
