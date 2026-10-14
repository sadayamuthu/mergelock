// Package rules holds one conflict detector per file. Each rule's ID is the
// verdict class it emits.
package rules

import (
	"fmt"
	"sort"

	"github.com/sadayamuthu/mergelock/internal/engine"
)

// All returns every v0.1 rule.
func All() []engine.Rule {
	return []engine.Rule{Textual{}, AddAdd{}, ModifyDelete{}, SignatureCaller{}}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// plural formats "1 call site" / "2 call sites".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
