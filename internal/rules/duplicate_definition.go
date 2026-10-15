package rules

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// DuplicateDefinition: both branches add the same helper, either under the
// same name in different files or with near-identical bodies.
type DuplicateDefinition struct{}

func (DuplicateDefinition) ID() string { return "duplicate_definition" }

var dupKinds = map[model.Kind]bool{model.KindFunction: true, model.KindMethod: true, model.KindClass: true}

func (DuplicateDefinition) Run(rc *engine.RuleContext) []verdict.Finding {
	if !rc.Primary {
		return nil
	}
	this, other := rc.This.Delta, rc.Other.Delta
	if this == nil || other == nil {
		return nil
	}
	threshold := rc.Config.RuleFloat("duplicate_definition", "similarity", 0.85)
	// One finding per symbol, listing every place each side adds it.
	type group struct {
		kind, symbol    string
		sim             float64
		mine, theirs    []verdict.Location
		mineAt, theirAt map[verdict.Location]bool
	}
	var order []string
	groups := map[string]*group{}
	for _, ka := range sortedKeys(this.Added) {
		a := this.Added[ka]
		if !dupKinds[a.Kind] || isNonProductionPath(a.File) {
			continue
		}
		for _, kb := range sortedKeys(other.Added) {
			b := other.Added[kb]
			if b.Kind != a.Kind || isNonProductionPath(b.File) {
				continue
			}
			kind, sim := "", 0.0
			switch {
			// Methods are class-scoped (Class.method); a same-named class already reports.
			case a.Name == b.Name && a.File != b.File && a.Kind != model.KindMethod:
				kind = "same_name"
			case a.Name != b.Name && len(a.Shingles) > 0 && len(b.Shingles) > 0:
				if s := model.Jaccard(a.Shingles, b.Shingles); s >= threshold {
					kind, sim = "similar_body", math.Round(s*100)/100
				}
			}
			if kind == "" {
				continue
			}
			symbol := a.Name
			if a.Name != b.Name {
				symbol = a.Name + " / " + b.Name
			}
			id := kind + "\x00" + symbol
			g, ok := groups[id]
			if !ok {
				g = &group{kind: kind, symbol: symbol, mineAt: map[verdict.Location]bool{}, theirAt: map[verdict.Location]bool{}}
				groups[id] = g
				order = append(order, id)
			}
			g.sim = math.Max(g.sim, sim)
			if la := (verdict.Location{File: a.File, Line: a.Line}); !g.mineAt[la] {
				g.mineAt[la] = true
				g.mine = append(g.mine, la)
			}
			if lb := (verdict.Location{File: b.File, Line: b.Line}); !g.theirAt[lb] {
				g.theirAt[lb] = true
				g.theirs = append(g.theirs, lb)
			}
		}
	}
	var out []verdict.Finding
	for _, id := range order {
		g := groups[id]
		nameA, nameB := g.symbol, g.symbol
		if a, b, ok := strings.Cut(g.symbol, " / "); ok {
			nameA, nameB = a, b
		}
		out = append(out, verdict.Finding{
			Class: "duplicate_definition", Kind: g.kind,
			Severity: verdict.SeverityWarning, Confidence: verdict.ConfidenceMedium,
			Symbol: g.symbol, ChangedIn: rc.This.Branch, UsedIn: rc.Other.Branch,
			Duplicates: append(append([]verdict.Location(nil), g.mine...), g.theirs...),
			Similarity: g.sim,
			SuggestedAction: fmt.Sprintf("%s adds %s at %s and %s adds %s at %s; keep one definition and point callers at it.",
				rc.This.Branch, nameA, joinLocations(g.mine), rc.Other.Branch, nameB, joinLocations(g.theirs)),
		})
	}
	return out
}

func joinLocations(ls []verdict.Location) string {
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = fmt.Sprintf("%s:%d", l.File, l.Line)
	}
	return strings.Join(parts, ", ")
}

// nonProductionDirs hold tests, fixtures, examples and docs, where the same
// conventional names (main, GET, get_client, test helpers) recur by design.
var nonProductionDirs = map[string]bool{
	"test": true, "tests": true, "__tests__": true, "spec": true, "specs": true, "e2e": true,
	"fixture": true, "fixtures": true, "__fixtures__": true, "__mocks__": true, "mocks": true, "testdata": true,
	"example": true, "examples": true, "sample": true, "samples": true, "demo": true, "demos": true,
	"doc": true, "docs": true, "docs_src": true, "playground": true, "playgrounds": true,
	"bench": true, "benchmark": true, "benchmarks": true,
}

var nonProductionFile = regexp.MustCompile(`^(test_.*\.py|.*_test\.py|conftest\.py|.*\.(test|spec|bench|stories)\.[cm]?[jt]sx?)$`)

func isNonProductionPath(p string) bool {
	parts := strings.Split(p, "/")
	for _, d := range parts[:len(parts)-1] {
		if nonProductionDirs[d] {
			return true
		}
	}
	return nonProductionFile.MatchString(parts[len(parts)-1])
}
