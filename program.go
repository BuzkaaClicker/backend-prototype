package buzza

import (
	"context"
	"errors"
)

var ErrProgramNotFound = errors.New("program not found")

type Program struct {
	Id     int
	Type   string
	OS     string
	Arch   string
	Branch string
	Files  []ProgramFile
}

// Single program file e.g. installer, config.yml, buzkaaclickeragent.dll.
type ProgramFile struct {
	// Relative file path in BuzkaaClicker directory.
	Path string
	// Download url.
	DownloadUrl string
	// File sha256 hash.
	Hash string
}

type ProgramStore interface {
	// Get latest program files matching specified arguments.
	LatestProgramFiles(ctx context.Context, fileType string,
		os string, arch string, branch string) ([]ProgramFile, error)
}
