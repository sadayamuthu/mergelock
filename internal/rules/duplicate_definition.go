package rules

import (
	"fmt"
	"math"

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
	var out []verdict.Finding
	for _, ka := range sortedKeys(this.Added) {
		a := this.Added[ka]
		if !dupKinds[a.Kind] {
			continue
		}
		for _, kb := range sortedKeys(other.Added) {
			b := other.Added[kb]
			if b.Kind != a.Kind {
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
			out = append(out, verdict.Finding{
				Class: "duplicate_definition", Kind: kind,
				Severity: verdict.SeverityWarning, Confidence: verdict.ConfidenceMedium,
				Symbol: symbol, ChangedIn: rc.This.Branch, UsedIn: rc.Other.Branch,
				Duplicates: []verdict.Location{{File: a.File, Line: a.Line}, {File: b.File, Line: b.Line}},
				Similarity: sim,
				SuggestedAction: fmt.Sprintf("%s adds %s at %s:%d and %s adds %s at %s:%d; keep one definition and point callers at it.",
					rc.This.Branch, a.Name, a.File, a.Line, rc.Other.Branch, b.Name, b.File, b.Line),
			})
		}
	}
	return out
}
