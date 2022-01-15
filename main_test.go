package main

import (
	"context"
	"flag"
	"testing"

	"github.com/sirupsen/logrus"
)

var createTestApp func(configureServer... func(app *app)) *app

func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Short() {
		shutdownInfra := setupTestIntegrationInfra()
		defer shutdownInfra()
	}

	m.Run()
}

func setupTestIntegrationInfra() (shutdown func()) {
	discordConfig := discordConfig{
		clientId:     "mock_client_id",
		clientSecret: "mock_client_secret",
		redirectUri:  "mock_redirect_uri",
	}
	bdb, db, shutdownDbs := createTestDatabases()
	createTestApp = func(configureServer... func(app *app)) *app {
		switch len(configureServer) {
		case 0:
			return newApp(context.Background(), bdb, db, discordConfig, true, nil)
		case 1:
			return newApp(context.Background(), bdb, db, discordConfig, true, configureServer[0])
		default:
			logrus.Fatalf("invalid configure server handler count: %d (must be 0 or 1)\n", len(configureServer))
			return nil
		}
	}

	return func() {
		shutdownDbs()
	}
}
