package rules

import (
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// Textual reports overlapping line edits found by git merge-tree.
type Textual struct{}

func (Textual) ID() string { return "textual" }

func (Textual) Run(rc *engine.RuleContext) []verdict.Finding {
	return mergeTreeFindings(rc, "textual")
}
