// Package engine runs a check: it resolves both branches, gathers what each
// changed, runs the rules and finalizes the verdict. It never touches the
// network and never writes to the repository.
package engine

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sadayamuthu/mergelock/internal/cache"
	"github.com/sadayamuthu/mergelock/internal/config"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/lang/langs"
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
	Delta  *Delta
}

type RuleContext struct {
	This, Other *Side
	Primary     bool // true for the (A, B) pass, false for (B, A)
	MergeTree   *gitx.MergeTreeResult
	Config      *config.Config

	loader *Loader
	reg    *lang.Registry
}

type Options struct {
	Config   *config.Config // nil means defaults
	Rules    []Rule
	Registry *lang.Registry // nil means langs.Default()
	NoCache  bool           // ignore and do not write the parse cache
}

// Check compares branchA and branchB against their merge base.
func Check(ctx context.Context, repo *gitx.Repo, branchA, branchB string, opts Options) (*verdict.Verdict, error) {
	start := time.Now()
	cfg := opts.Config
	if cfg == nil {
		cfg = config.Default()
	}
	reg := opts.Registry
	if reg == nil {
		reg = langs.Default()
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
	base := bases[0]
	v := &verdict.Verdict{Base: base, Branches: []string{branchA, branchB}}
	if len(bases) > 1 {
		v.Notes = append(v.Notes, "multiple_merge_bases")
	}
	mt, err := repo.MergeTree(branchA, branchB)
	if err != nil {
		return nil, err
	}
	changesA, err := repo.DiffTree(base, commitA)
	if err != nil {
		return nil, err
	}
	changesB, err := repo.DiffTree(base, commitB)
	if err != nil {
		return nil, err
	}
	blobs, err := repo.NewBlobReader()
	if err != nil {
		return nil, err
	}
	defer blobs.Close()
	loader := NewLoader(repo, blobs, cache.Open(repo.CommonDir, opts.NoCache), reg)
	deltaA, err := computeDelta(loader, base, commitA, changesA)
	if err != nil {
		return nil, err
	}
	deltaB, err := computeDelta(loader, base, commitB, changesB)
	if err != nil {
		return nil, err
	}
	a := &Side{Branch: branchA, Commit: commitA, Delta: deltaA}
	b := &Side{Branch: branchB, Commit: commitB, Delta: deltaB}

	enabled := enabledRules(opts.Rules, cfg)
	for _, r := range enabled {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, rc := range []*RuleContext{
			{This: a, Other: b, Primary: true, MergeTree: mt, Config: cfg, loader: loader, reg: reg},
			{This: b, Other: a, Primary: false, MergeTree: mt, Config: cfg, loader: loader, reg: reg},
		} {
			v.Findings = append(v.Findings, r.Run(rc)...)
		}
	}
	downgradePartial(v.Findings, loader.Partial)
	v.Coverage = verdict.Coverage{
		Languages:   sortedSet(loader.Languages),
		Unsupported: sortedSet(loader.Unsupported),
		Partial:     sortedSet(loader.Partial),
	}
	v.Stats.RulesRun = len(enabled)
	v.Stats.SymbolsCompared = deltaA.SymbolsCompared + deltaB.SymbolsCompared
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

// downgradePartial lowers confidence by one level for findings that involve
// a file the parser could not fully read. Git-derived findings are exempt.
func downgradePartial(fs []verdict.Finding, partial map[string]bool) {
	for i := range fs {
		if fs[i].FromGit {
			continue
		}
		for _, p := range fs[i].Files() {
			if partial[p] {
				fs[i].Confidence = fs[i].Confidence.Downgrade()
				break
			}
		}
	}
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
