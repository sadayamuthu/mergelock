package rules

import (
	"fmt"

	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// ModifyDelete reports one branch deleting what the other changes or starts
// using. File-level cases come from merge-tree; symbol-level cases from deltas.
type ModifyDelete struct{}

func (ModifyDelete) ID() string { return "modify_delete" }

func (ModifyDelete) Run(rc *engine.RuleContext) []verdict.Finding {
	out := mergeTreeFindings(rc, "modify_delete")
	this, other := rc.This.Delta, rc.Other.Delta
	if this == nil || other == nil {
		return out
	}
	gitFiles := fileConflictPaths(rc) // already reported by merge-tree
	for _, key := range sortedKeys(this.Removed) {
		s := this.Removed[key]
		renamedTo := ""
		if nk, ok := this.Renamed[key]; ok {
			renamedTo = model.ScopeName(nk)
		}
		deletedAt := &verdict.Location{File: s.File, Line: s.Line}

		ch, modified := other.SigChanged[key]
		if !modified {
			ch, modified = other.BodyChanged[key]
		}
		if modified && !gitFiles[s.File] {
			out = append(out, verdict.Finding{
				Class: "modify_delete", Kind: "deleted_symbol_modified",
				Severity: verdict.SeverityError, Confidence: verdict.ConfidenceCertain,
				Symbol: s.Name, ChangedIn: rc.This.Branch, ChangedAt: deletedAt, RenamedTo: renamedTo,
				UsedIn: rc.Other.Branch, ModifiedAt: &verdict.Location{File: ch.New.File, Line: ch.New.Line},
				SuggestedAction: fmt.Sprintf("%s deletes %s but %s edits it; decide whether it survives, then rebase %s.",
					rc.This.Branch, s.Name, rc.Other.Branch, rc.Other.Branch),
			})
		}

		sites, conf := newCallSites(rc, s, func(ref model.Reference) (verdict.Confidence, bool) {
			if ref.Spread {
				return verdict.ConfidenceMedium, true
			}
			return verdict.ConfidenceCertain, true
		})
		if len(sites) == 0 {
			continue
		}
		action := fmt.Sprintf("%s deletes %s; remove the %s in %s or restore the symbol before merging.",
			rc.This.Branch, s.Name, plural(len(sites), "new call site"), rc.Other.Branch)
		if renamedTo != "" {
			action = fmt.Sprintf("%s renames %s to %s; update the %s in %s to use %s.",
				rc.This.Branch, s.Name, renamedTo, plural(len(sites), "call site"), rc.Other.Branch, renamedTo)
		}
		out = append(out, verdict.Finding{
			Class: "modify_delete", Kind: "deleted_symbol_referenced",
			Severity: verdict.SeverityError, Confidence: conf,
			Symbol: s.Name, ChangedIn: rc.This.Branch, ChangedAt: deletedAt, RenamedTo: renamedTo,
			UsedIn: rc.Other.Branch, CallSites: sites, SuggestedAction: action,
		})
	}
	return out
}

// fileConflictPaths lists paths merge-tree reports as modify/delete. Both
// passes need it: the file finding is emitted only in the primary pass.
func fileConflictPaths(rc *engine.RuleContext) map[string]bool {
	out := map[string]bool{}
	if rc.MergeTree == nil {
		return out
	}
	for _, m := range rc.MergeTree.Messages {
		if classifyConflict(m.Message) == "modify_delete" {
			for _, p := range m.Paths {
				out[p] = true
			}
		}
	}
	return out
}
