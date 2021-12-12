package main

import (
	"context"
	"flag"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

var db *bun.DB

func TestMain(m *testing.M) {
	flag.Parse()

	var shutdownDb func()
	if !testing.Short() {
		logrus.Infoln("Starting db")
		var err error
		db, shutdownDb, err = createTestDb()
		if err != nil {
			logrus.WithError(err).Fatalln("Could not create test database.")
			return
		}
		defer shutdownDb()
	}

	m.Run()
}

func TestVersionLatest(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	_, err := db.NewCreateTable().Model((*Version)(nil)).Exec(ctx)
	assert.NoError(err)

	_, err = db.NewInsert().Model(&[]Version{
		{Number: 1, OS: "macOS", Architecture: "amd64", Branch: "stable"},
		{Number: 2, OS: "macOS", Architecture: "amd64", Branch: "stable"},
		{Number: 1, OS: "Windows", Architecture: "amd64", Branch: "stable"},
		{Number: 2, OS: "Windows", Architecture: "amd64", Branch: "stable"},
		{Number: 3, OS: "Windows", Architecture: "amd64", Branch: "beta"},
		{Number: 3, OS: "Windows", Architecture: "arm64", Branch: "beta"},
	}).Exec(ctx)
	assert.NoError(err)

	controller := VersionController{Repo: PgVersionRepo{DB: db}}

	req := httptest.NewRequest("GET", "/version/latest", nil)
	app := fiber.New()
	app.Get("/version/latest", controller.ServeLatestVersions)
	resp, err := app.Test(req)
	assert.NoError(err)
	defer resp.Body.Close()

	assert.Equal(resp.Header.Get("Content-Type"), fiber.MIMEApplicationJSON, "Invalid content type")
	const validResponseBody = `[{"id":2,"number":2,"os":"macOS","architecture":"amd64","branch":"stable"},` +
		`{"id":5,"number":3,"os":"Windows","architecture":"amd64","branch":"beta"},` +
		`{"id":4,"number":2,"os":"Windows","architecture":"amd64","branch":"stable"},` +
		`{"id":6,"number":3,"os":"Windows","architecture":"arm64","branch":"beta"}]`
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(err)
	assert.Equal(validResponseBody, string(body), "Response body not equal")
}
