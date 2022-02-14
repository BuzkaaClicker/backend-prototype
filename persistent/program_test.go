package persistent

import (
	"context"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/stretchr/testify/assert"
)

func TestProgramStore(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	db := PgOpenTest(ctx)
	defer db.Close()

	exampleFiles := []ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "256"}}
	_, err := db.NewInsert().Model(&[]Program{
		{Type: "installer", OS: "macOS", Arch: "x86-64", Branch: "stable",
			Files: []ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "499"}}},
		{Type: "installer", OS: "macOS", Arch: "x86-64", Branch: "beta", Files: exampleFiles},
		{Type: "installer", OS: "macOS", Arch: "arm64", Branch: "stable", Files: exampleFiles},
		{Type: "installer", OS: "Windows", Arch: "x86-64", Branch: "stable", Files: exampleFiles},
		{Type: "installer", OS: "Windows", Arch: "arm8", Branch: "alpha", Files: exampleFiles},
		{Type: "clicker", OS: "macOS", Arch: "x86-64", Branch: "stable",
			Files: []ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "1"}}},
	}).Exec(ctx)
	if !assert.NoError(err) {
		return
	}

	store := ProgramStore{DB: db}

	exampleDFiles := []buzza.ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "256"}}
	cases := []struct {
		fileType      string
		os            string
		arch          string
		branch        string
		expectedFiles []buzza.ProgramFile
	}{
		{"installer", "macOS", "x86-64", "stable", []buzza.ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "499"}}},
		{"installer", "macOS", "x86-64", "beta", exampleDFiles},
		{"installer", "macOS", "arm64", "stable", exampleDFiles},
		{"installer", "Windows", "x86-64", "stable", exampleDFiles},
		{"installer", "Windows", "arm8", "alpha", exampleDFiles},
		{"clicker", "macOS", "x86-64", "stable", []buzza.ProgramFile{{Path: "installer.pkg", DownloadUrl: "https://buzkaaclicker.pl/sample", Hash: "1"}}},
	}
	for _, c := range cases {
		pf, err := store.LatestProgramFiles(ctx, c.fileType, c.os, c.arch, c.branch)
		if !assert.NoError(err) {
			continue
		}
		assert.Equal(c.expectedFiles, pf)
	}
}
