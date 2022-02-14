package main

import (
	"context"
	"flag"
	"log/syslog"
	"os"
	"os/signal"
	"time"

	"github.com/buzkaaclicker/buzza/discord"
	"github.com/buzkaaclicker/buzza/persistent"
	"github.com/buzkaaclicker/buzza/transport/rest"
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
	userStore := &persistent.UserStore{DB: db}
	profileStore := &persistent.ProfileStore{DB: db}
	activityStore := &persistent.ActivityStore{DB: db}
	sessionStore := &persistent.SessionStore{Buntdb: bdb, ActivityStore: activityStore}
	sessionStore.CreateIndexes()

	authController := rest.AuthController{
		CreateDiscordOAuthUrl: discordConfig.oauthUrlFactory,
		ExchangeAccessToken:   discordConfig.accessTokenExchanger,
		UserMeProvider:        discord.RestUserMeProvider,
		GuildMemberAdd:        discordConfig.guildMemberAdd,
		SessionStore:          sessionStore,
		UserStore:             userStore,
	}

	programStore := &persistent.ProgramStore{DB: db}
	programController := rest.ProgramController{Store: programStore}
	profileController := rest.ProfileController{Store: profileStore}
	activityController := rest.ActivityController{Store: activityStore}
	sessionController := rest.SessionController{Store: sessionStore}

	server := fiber.New()
	server.Use(rest.LogHandler())

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

	requestAuthorizer := rest.RequestAuthorizer(sessionStore, userStore)
	api.Get("/status", monitor.New())
	authController.InstallTo(api)
	programController.InstallTo(api)
	profileController.InstallTo(api)
	activityController.InstallTo(requestAuthorizer, api)
	sessionController.InstallTo(requestAuthorizer, api)

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
		return nil
		// return server.Shutdown()
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
	pg := persistent.PgOpen(context.Background(), pgDsn)
	if debug {
		pg.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}
	defer pg.DB.Close()
	defer pg.Close()

	discordConfig := discordConfigFromEnv()

	logrus.Infoln("Starting listening... To shut down use ^C")
	shutdown := listenAndServe(context.Background(), bdb, pg, discordConfig, debug)

	awaitInterruption()

	logrus.Infoln("Shutting down...")
	err = shutdown()
	if err != nil {
		logrus.WithError(err).Warningln("Fiber shutdown failed.")
	}
	logrus.Exit(0)
}
