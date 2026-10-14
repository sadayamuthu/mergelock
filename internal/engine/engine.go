// Package engine runs a check: it resolves both branches, gathers what each
// changed, runs the rules and finalizes the verdict. It never touches the
// network and never writes to the repository.
package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/sadayamuthu/mergelock/internal/config"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// Rule is one conflict detector. Run is called twice per check, once per
// direction; symmetric rules emit only when rc.Primary is true.
type Rule interface {
	ID() string
	Run(rc *RuleContext) []verdict.Finding
}

// Side is one branch of the pair being checked.
type Side struct {
	Branch string // name as the user gave it
	Commit string // full SHA
}

type RuleContext struct {
	This, Other *Side
	Primary     bool // true for the (A, B) pass, false for (B, A)
	MergeTree   *gitx.MergeTreeResult
	Config      *config.Config
}

type Options struct {
	Config  *config.Config // nil means defaults
	Rules   []Rule
	NoCache bool // ignore and do not write the parse cache
}

// Check compares branchA and branchB against their merge base.
func Check(ctx context.Context, repo *gitx.Repo, branchA, branchB string, opts Options) (*verdict.Verdict, error) {
	start := time.Now()
	cfg := opts.Config
	if cfg == nil {
		cfg = config.Default()
	}
	commitA, err := repo.ResolveCommit(branchA)
	if err != nil {
		return nil, err
	}
	commitB, err := repo.ResolveCommit(branchB)
	if err != nil {
		return nil, err
	}
	bases, err := repo.MergeBases(commitA, commitB)
	if err != nil {
		return nil, err
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("no merge base between %s and %s", branchA, branchB)
	}
	v := &verdict.Verdict{Base: bases[0], Branches: []string{branchA, branchB}}
	if len(bases) > 1 {
		v.Notes = append(v.Notes, "multiple_merge_bases")
	}
	mt, err := repo.MergeTree(branchA, branchB)
	if err != nil {
		return nil, err
	}
	a := &Side{Branch: branchA, Commit: commitA}
	b := &Side{Branch: branchB, Commit: commitB}

	enabled := enabledRules(opts.Rules, cfg)
	for _, r := range enabled {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		v.Findings = append(v.Findings, r.Run(&RuleContext{This: a, Other: b, Primary: true, MergeTree: mt, Config: cfg})...)
		v.Findings = append(v.Findings, r.Run(&RuleContext{This: b, Other: a, Primary: false, MergeTree: mt, Config: cfg})...)
	}
	v.Stats.RulesRun = len(enabled)
	verdict.Finalize(v)
	v.Stats.DurationMS = time.Since(start).Milliseconds()
	return v, nil
}

func enabledRules(all []Rule, cfg *config.Config) []Rule {
	var out []Rule
	for _, r := range all {
		if cfg.RuleEnabled(r.ID()) {
			out = append(out, r)
		}
	}
	return out
}
