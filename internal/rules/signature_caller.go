package rules

import (
	"fmt"

	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// SignatureCaller: this branch changes the signature of S; the other branch
// adds calls to S that fit the old signature but not the new one.
type SignatureCaller struct{}

func (SignatureCaller) ID() string { return "signature_caller" }

func (SignatureCaller) Run(rc *engine.RuleContext) []verdict.Finding {
	this, other := rc.This.Delta, rc.Other.Delta
	if this == nil || other == nil {
		return nil
	}
	var out []verdict.Finding
	for _, key := range sortedKeys(this.SigChanged) {
		ch := this.SigChanged[key]
		if ch.Old.Kind != model.KindFunction && ch.Old.Kind != model.KindMethod {
			continue
		}
		sites, conf := newCallSites(rc, ch.Old, func(ref model.Reference) (verdict.Confidence, bool) {
			if !model.Accepts(ch.Old.Signature, ref.ArgCount, ref.Keywords, ref.Spread) ||
				model.Accepts(ch.New.Signature, ref.ArgCount, ref.Keywords, ref.Spread) {
				return "", false
			}
			if ref.Spread {
				return verdict.ConfidenceMedium, true
			}
			return verdict.ConfidenceCertain, true
		})
		if len(sites) == 0 {
			continue
		}
		out = append(out, verdict.Finding{
			Class:        "signature_caller",
			Severity:     verdict.SeverityError,
			Confidence:   conf,
			Symbol:       ch.Old.Name,
			ChangedIn:    rc.This.Branch,
			ChangedAt:    &verdict.Location{File: ch.New.File, Line: ch.New.Line},
			OldSignature: ch.Old.Signature.Text,
			NewSignature: ch.New.Signature.Text,
			UsedIn:       rc.Other.Branch,
			CallSites:    sites,
			SuggestedAction: fmt.Sprintf("Update %s in %s to the new signature, or merge %s first and rebase %s.",
				plural(len(sites), "call site"), rc.Other.Branch, rc.Other.Branch, rc.This.Branch),
		})
	}
	return out
}

// siteFilter decides whether a reference that calls the symbol counts as a
// call site, and caps its confidence at the returned ceiling.
type siteFilter func(ref model.Reference) (ceiling verdict.Confidence, ok bool)

// newCallSites collects the other branch's new references that call sym,
// keeping those the filter accepts (nil keeps all). It returns their
// locations and the highest confidence among them.
func newCallSites(rc *engine.RuleContext, sym model.Symbol, keep siteFilter) ([]verdict.Location, verdict.Confidence) {
	var sites []verdict.Location
	var conf verdict.Confidence
	for _, ref := range rc.Other.Delta.NewRefs {
		c, ok := rc.Matches(rc.Other, ref, sym)
		if !ok {
			continue
		}
		if keep != nil {
			ceiling, ok := keep(ref)
			if !ok {
				continue
			}
			c = verdict.MinConfidence(c, ceiling)
		}
		sites = append(sites, verdict.Location{File: ref.File, Line: ref.Line, Text: ref.Text})
		conf = verdict.MaxConfidence(conf, c)
	}
	return sites, conf
}
