package rules

import "testing"

func TestIsNonProductionPath(t *testing.T) {
	for p, want := range map[string]bool{
		"src/strings.ts":                         false,
		"fastapi/dependencies/utils.py":          false,
		"packages/zod/src/v4/classic/schemas.ts": false,
		"src/testing.ts":                         false,
		"src/latest/index.ts":                    false,
		"tests/test_tutorial/test_body.py":       true,
		"packages/astro/test/fixtures/a/b.ts":    true,
		"src/__tests__/auth.ts":                  true,
		"src/auth.test.ts":                       true,
		"src/auth.spec.tsx":                      true,
		"pkg/test_utils.py":                      true,
		"pkg/utils_test.py":                      true,
		"conftest.py":                            true,
		"docs_src/body/tutorial001.py":           true,
		"examples/minimal/server.ts":             true,
		"src/Button.stories.tsx":                 true,
	} {
		if got := isNonProductionPath(p); got != want {
			t.Errorf("isNonProductionPath(%q) = %v, want %v", p, got, want)
		}
	}
}
