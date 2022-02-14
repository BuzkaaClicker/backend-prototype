package mock

import (
	"context"

	"github.com/buzkaaclicker/buzza"
)

type ProgramStore struct {
	LatestProgramFilesFn func(ctx context.Context,
		fileType string, os string, arch string, branch string) ([]buzza.ProgramFile, error)
}

func (s ProgramStore) LatestProgramFiles(ctx context.Context,
	fileType string, os string, arch string, branch string) ([]buzza.ProgramFile, error) {
	return s.LatestProgramFilesFn(ctx, fileType, os, arch, branch)
}
