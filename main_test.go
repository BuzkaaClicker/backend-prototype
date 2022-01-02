package main

import (
	"context"
	"flag"
	"testing"
)

var createTestApp func() *app

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
	createTestApp = func() *app {
		return newApp(context.Background(), bdb, db, discordConfig)
	}

	return func() {
		shutdownDbs()
	}
}
