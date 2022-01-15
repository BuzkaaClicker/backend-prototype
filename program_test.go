package main

import (
	"context"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestDownloadProgram(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	app := createTestApp()

	exampleFile := []ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "256"}}
	_, err := app.db.NewInsert().Model(&[]Program{
		{Type: "installer", OS: "macOS", Arch: "x86-64", Branch: "stable",
			Files: []ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "499"}}},
		{Type: "installer", OS: "macOS", Arch: "x86-64", Branch: "beta", Files: exampleFile},
		{Type: "installer", OS: "macOS", Arch: "arm64", Branch: "stable", Files: exampleFile},
		{Type: "installer", OS: "Windows", Arch: "x86-64", Branch: "stable", Files: exampleFile},
		{Type: "installer", OS: "Windows", Arch: "arm8", Branch: "alpha", Files: exampleFile},
		{Type: "clicker", OS: "macOS", Arch: "x86-64", Branch: "stable",
			Files: []ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "1"}}},
	}).Exec(ctx)
	assert.NoError(err)

	cases := []struct {
		url  string
		body string
	}{
		{"/api/download/installer?os=macOS&arch=x86-64&branch=stable",
			`[{"path":"installer.pkg","download_url":"https://buzkaaclicker.pl/sample","hash":"499"}]`},
		{"/api/download/clicker?os=macOS&arch=x86-64&branch=stable",
			`[{"path":"installer.pkg","download_url":"https://buzkaaclicker.pl/sample","hash":"1"}]`},
		{"/api/download/clicker?os=macOS&arch=arm64&branch=stable", `{"error_message":"Not Found"}`},
		{"/api/download/clicker?os=macOS&arch=x86-64&branch=unstable", `{"error_message":"Not Found"}`},
		{"/api/download/clicker?os=macOSes&arch=x86-64&branch=stable", `{"error_message":"Not Found"}`},
		{"/api/download/clicker?os=Windows&arch=x86-64&branch=stable", `{"error_message":"Not Found"}`},
		{"/api/download/installer?os=Windows&arch=x86-64&branch=stable",
			`[{"path":"installer.pkg","download_url":"https://buzkaaclicker.pl/sample","hash":"256"}]`},
	}

	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.url, nil)
		resp, err := app.server.Test(req)
		assert.NoError(err)
		defer resp.Body.Close()

		assert.Equal(resp.Header.Get("Content-Type"), fiber.MIMEApplicationJSON, "Invalid content type")
		body, err := ioutil.ReadAll(resp.Body)
		assert.NoError(err)
		assert.Equal(tc.body, string(body), "Response body not equal")
	}
}
