package rest

import (
	"context"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/mock"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestDownloadProgram(t *testing.T) {
	assert := assert.New(t)

	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	programStore := mock.ProgramStore{}
	controller := ProgramController{
		Store: &programStore,
	}
	controller.InstallTo(app)

	cases := []struct {
		url   string
		body  string
		files []buzza.ProgramFile
	}{
		{"/download/installer?os=macOS&arch=x86-64&branch=stable",
			`[{"path":"installer.pkg","downloadUrl":"https://buzkaaclicker.pl/sample","hash":"499"}]`,
			[]buzza.ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "499"}}},
		{"/download/clicker?os=macOS&arch=x86-64&branch=stable",
			`[{"path":"installer.pkg","downloadUrl":"https://buzkaaclicker.pl/sample","hash":"1"}]`,
			[]buzza.ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "1"}}},
		{"/download/clicker?os=macOS&arch=arm64&branch=stable", `{"error_message":"Not Found"}`, nil},
		{"/download/clicker?os=macOS&arch=x86-64&branch=unstable", `{"error_message":"Not Found"}`, nil},
		{"/download/clicker?os=macOSes&arch=x86-64&branch=stable", `{"error_message":"Not Found"}`, nil},
		{"/download/clicker?os=Windows&arch=x86-64&branch=stable", `{"error_message":"Not Found"}`, nil},
		{"/download/installer?os=Windows&arch=x86-64&branch=stable",
			`[{"path":"installer.pkg","downloadUrl":"https://buzkaaclicker.pl/sample","hash":"256"}]`,
			[]buzza.ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "256"}}},
	}

	for _, tc := range cases {
		programStore.LatestProgramFilesFn = func(ctx context.Context,
			fileType string, os string, arch string, branch string) ([]buzza.ProgramFile, error) {
			if tc.files == nil {
				return nil, buzza.ErrProgramNotFound
			}
			return tc.files, nil
		}

		req := httptest.NewRequest("GET", tc.url, nil)
		resp, err := app.Test(req)
		assert.NoError(err)
		defer resp.Body.Close()

		assert.Equal(resp.Header.Get("Content-Type"), fiber.MIMEApplicationJSON, "Invalid content type")
		body, err := ioutil.ReadAll(resp.Body)
		assert.NoError(err)
		assert.Equal(tc.body, string(body), "Response body not equal")
	}
}
