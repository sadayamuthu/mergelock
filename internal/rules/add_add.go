package rules

import (
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// AddAdd reports both branches creating the same path with different content.
type AddAdd struct{}

func (AddAdd) ID() string { return "add_add" }

func (AddAdd) Run(rc *engine.RuleContext) []verdict.Finding {
	return mergeTreeFindings(rc, "add_add")
}
