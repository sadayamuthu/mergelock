package engine

import (
	"context"

	"github.com/sadayamuthu/mergelock/internal/config"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// CheckDir is the entry point shared by the CLI and the MCP server: it opens
// the repository containing dir, loads its .mergelock.yml into opts.Config
// and runs Check. An empty branchA means the checked-out branch; an empty
// branchB means the configured target.
func CheckDir(ctx context.Context, dir, branchA, branchB string, opts Options) (*verdict.Verdict, error) {
	repo, err := gitx.Open(dir)
	if err != nil {
		return nil, err
	}
	if opts.Config, err = config.Load(repo.Dir); err != nil {
		return nil, err
	}
	if branchA == "" {
		if branchA, err = repo.CurrentBranch(); err != nil {
			return nil, err
		}
	}
	if branchB == "" {
		branchB = opts.Config.Target
	}
	return Check(ctx, repo, branchA, branchB, opts)
}
