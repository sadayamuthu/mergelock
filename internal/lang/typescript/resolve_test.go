package typescript

import "testing"

type mapFS map[string]string

func (m mapFS) Exists(p string) bool { _, ok := m[p]; return ok }
func (m mapFS) Read(p string) ([]byte, bool) {
	s, ok := m[p]
	return []byte(s), ok
}

func TestResolveModule(t *testing.T) {
	fs := mapFS{
		"src/auth.ts":           "",
		"src/auth/index.ts":     "",
		"src/lib/fmt.ts":        "",
		"src/esm.ts":            "",
		"src/widgets/index.tsx": "",
		"vendor/raw.js":         "",
		"tsconfig.json": `{
  // aliases for app code
  "compilerOptions": {
    "baseUrl": ".",
    "paths": { "@app/*": ["src/*"], "@fmt": ["src/lib/fmt.ts"], },
  },
  /* trailing */
}`,
	}
	a := New()
	cases := []struct {
		from, spec, want string
		ok               bool
	}{
		{"src/users.ts", "./auth", "src/auth.ts", true},
		{"src/api/users.ts", "../auth/index", "src/auth/index.ts", true},
		{"src/users.ts", "./widgets", "src/widgets/index.tsx", true},
		{"src/users.ts", "./esm.js", "src/esm.ts", true},
		{"src/users.ts", "../vendor/raw.js", "vendor/raw.js", true},
		{"src/users.ts", "@app/lib/fmt", "src/lib/fmt.ts", true},
		{"src/users.ts", "@fmt", "src/lib/fmt.ts", true},
		{"src/users.ts", "src/auth", "src/auth.ts", true}, // via explicit baseUrl
		{"src/users.ts", "react", "", false},
		{"src/users.ts", "./missing", "", false},
	}
	for _, c := range cases {
		got, ok := a.ResolveModule(c.from, c.spec, fs)
		if got != c.want || ok != c.ok {
			t.Errorf("ResolveModule(%q, %q) = %q, %v; want %q, %v", c.from, c.spec, got, ok, c.want, c.ok)
		}
	}
}

func TestResolveWithoutTSConfig(t *testing.T) {
	fs := mapFS{"src/auth.ts": ""}
	if _, ok := New().ResolveModule("src/x.ts", "src/auth", fs); ok {
		t.Error("bare spec without tsconfig baseUrl must not resolve")
	}
	if got, ok := New().ResolveModule("src/x.ts", "./auth", fs); !ok || got != "src/auth.ts" {
		t.Errorf("relative = %q, %v", got, ok)
	}
}
