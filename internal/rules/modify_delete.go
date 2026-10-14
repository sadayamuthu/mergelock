package rules

import (
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// ModifyDelete reports one branch deleting what the other changes. File-level
// cases come from merge-tree; symbol-level cases are added in Task 14.
type ModifyDelete struct{}

func (ModifyDelete) ID() string { return "modify_delete" }

func (ModifyDelete) Run(rc *engine.RuleContext) []verdict.Finding {
	return mergeTreeFindings(rc, "modify_delete")
}
