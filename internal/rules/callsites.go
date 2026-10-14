package rules

import (
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// siteFilter decides whether a reference that calls the symbol counts as a
// call site, and caps its confidence at the returned ceiling.
type siteFilter func(ref model.Reference) (ceiling verdict.Confidence, ok bool)

// newCallSites collects the other branch's new references that call sym,
// keeping those the filter accepts (nil keeps all). It returns their
// locations and the highest confidence among them.
func newCallSites(rc *engine.RuleContext, sym model.Symbol, keep siteFilter) ([]verdict.Location, verdict.Confidence) {
	var sites []verdict.Location
	var conf verdict.Confidence
	if rc.Other.Delta == nil {
		return nil, ""
	}
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
