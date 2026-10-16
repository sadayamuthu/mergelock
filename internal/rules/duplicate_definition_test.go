package rules

import "testing"

func TestIsNonProductionPath(t *testing.T) {
	for p, want := range map[string]bool{
		"src/strings.ts":                             false,
		"fastapi/dependencies/utils.py":              false,
		"packages/zod/src/v4/classic/schemas.ts":     false,
		"src/testing.ts":                             false,
		"src/latest/index.ts":                        false,
		"src/builder.ts":                             false,
		"src/distance.py":                            false,
		"src/generatedish/client.ts":                 false,
		"src/message_pb2x.py":                        false,
		"tests/test_tutorial/test_body.py":           true,
		"packages/astro/test/fixtures/a/b.ts":        true,
		"src/__tests__/auth.ts":                      true,
		"vendor/github.com/example/client.go":        true,
		"packages/app/node_modules/library/index.ts": true,
		"web/dist/assets/app.js":                     true,
		"build/output/main.ts":                       true,
		"pkg/generated/client.ts":                    true,
		"pkg/__generated__/schema.ts":                true,
		"types/api.d.ts":                             true,
		"src/api.generated.ts":                       true,
		"src/api.gen.ts":                             true,
		"src/proto/message_pb2.py":                   true,
		"src/proto/message_pb2_grpc.py":              true,
		"web/assets/app.min.js":                      true,
		"src/auth.test.ts":                           true,
		"src/auth.spec.tsx":                          true,
		"pkg/test_utils.py":                          true,
		"pkg/utils_test.py":                          true,
		"conftest.py":                                true,
		"docs_src/body/tutorial001.py":               true,
		"examples/minimal/server.ts":                 true,
		"src/Button.stories.tsx":                     true,
	} {
		if got := isNonProductionPath(p); got != want {
			t.Errorf("isNonProductionPath(%q) = %v, want %v", p, got, want)
		}
	}
}
