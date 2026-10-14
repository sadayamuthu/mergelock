package rules

import (
	"slices"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/config"
)

// Every registered rule must be configurable in .mergelock.yml.
func TestRegisteredRulesAreKnownToConfig(t *testing.T) {
	for _, r := range All() {
		if !slices.Contains(config.RuleIDs, r.ID()) {
			t.Errorf("rule %q missing from config.RuleIDs", r.ID())
		}
	}
}
