package main

import (
	"context"
	"flag"
	"log/syslog"
	"os"
	"os/signal"
	"time"

	"github.com/buzkaaclicker/backend/discord"
	"github.com/buzkaaclicker/backend/pgdb"
	"github.com/buzkaaclicker/backend/profile"
	"github.com/buzkaaclicker/backend/program"
	"github.com/buzkaaclicker/backend/rest"
	"github.com/buzkaaclicker/backend/user"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/sirupsen/logrus"
	logrusys "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/tidwall/buntdb"
	"github.com/uptrace/bun"
	_ "github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

func listenAndServe(
	ctx context.Context,
	bdb *buntdb.DB,
	db *bun.DB,
	discordConfig discordConfig,
	debug bool,
) func() error {
	userStore := user.Store{DB: db}
	profileStore := profile.Store{DB: db}
	sessionStore := user.SessionStore{Buntdb: bdb, UserStore: &userStore}

	authController := user.AuthController{
		DB:                    db,
		CreateDiscordOAuthUrl: discordConfig.oauthUrlFactory,
		ExchangeAccessToken:   discordConfig.accessTokenExchanger,
		UserMeProvider:        discord.RestUserMeProvider,
		GuildMemberAdd:        discordConfig.guildMemberAdd,
		SessionStore:          &sessionStore,
		UserStore:             &userStore,
	}

	programRepo := &program.PgRepo{DB: db}
	programController := program.Controller{Repo: programRepo}
	profileController := profile.Controller{Store: profileStore}

	server := fiber.New()
	server.Use(logHandler())

	api := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorHandler: rest.ErrorHandler,
	})

	allowOrigins := "https://buzkaaclicker.pl"
	if debug {
		allowOrigins += ", http://test.buzkaaclicker.pl:3000"
	}
	api.Use(cors.New(cors.Config{AllowOrigins: allowOrigins}))

	api.Get("/status", rest.CombineHandlers(
		sessionStore.Authorize, user.RequirePermissions(user.PermissionAdminDashboard), monitor.New()))
	authController.InstallTo(api)
	programController.InstallTo(api)
	profileController.InstallTo(api)
	server.Mount("/api/", api)

	server.Static("/", "./www/", fiber.Static{
		Browse: false,
		Index:  "index.html",
	})

	server.Use(rest.NotFoundHandler)

	var addr string
	if debug {
		addr = "127.0.0.1:2137"
	} else {
		addr = ":2137"
	}
	go server.Listen(addr)

	return func() error {
		return server.Shutdown()
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

func logHandler() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		rest.RequestLog(ctx).Infoln("Handling request.")
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
	db := pgdb.Open(context.Background(), pgDsn)
	if debug {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}
	defer db.DB.Close()
	defer db.Close()

	discordConfig := discordConfigFromEnv()

	logrus.Infoln("Starting listening... To shut down use ^C")
	shutdown := listenAndServe(context.Background(), bdb, db, discordConfig, debug)

	awaitInterruption()

	logrus.Infoln("Shutting down...")
	err = shutdown()
	if err != nil {
		logrus.WithError(err).Warningln("Fiber shutdown failed.")
	}
	logrus.Exit(0)
}
