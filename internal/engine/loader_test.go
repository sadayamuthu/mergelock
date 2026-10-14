package engine

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/cache"
	"github.com/sadayamuthu/mergelock/internal/gittest"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/lang/langs"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

func newLoader(t *testing.T, dir string, noCache bool) (*Loader, *gitx.Repo) {
	t.Helper()
	repo, err := gitx.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := repo.NewBlobReader()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { blobs.Close() })
	return NewLoader(repo, blobs, cache.Open(repo.CommonDir, noCache), langs.Default()), repo
}

const authBase = `export function login(name: string, email: string): string {
  return name + email;
}

export function formatDate(d: Date): string {
  return "" + d;
}

export function old(): number {
  return 1;
}
`

const authA = `export function login(opts: { name: string }): string {
  return opts.name;
}

export function formatDay(d: Date): string {
  return "" + d;
}

export function added(): number {
  return 2;
}
`

func TestComputeDelta(t *testing.T) {
	r := gittest.New(t)
	r.Write("src/auth.ts", authBase)
	r.Write("src/users.ts", "export function list(): string[] {\n  return [];\n}\n")
	r.Write("README.md", "hi\n")
	base := r.Commit("base")
	r.Branch("a")
	r.Write("src/auth.ts", authA)
	r.Write("src/users.ts", "import { login } from \"./auth\";\nexport function list(): string[] {\n  return [login({ name: \"x\" })];\n}\n")
	r.Write("README.md", "hello\n")
	head := r.Commit("a")

	l, repo := newLoader(t, r.Dir, false)
	changes, err := repo.DiffTree(base, head)
	if err != nil {
		t.Fatal(err)
	}
	d, err := computeDelta(l, base, head, changes)
	if err != nil {
		t.Fatal(err)
	}
	ch, ok := d.SigChanged["src/auth.ts::login"]
	if !ok || len(ch.Old.Signature.Params) != 2 || len(ch.New.Signature.Params) != 1 {
		t.Errorf("SigChanged[login] = %+v, %v", ch, ok)
	}
	if _, ok := d.BodyChanged["src/auth.ts::login"]; !ok {
		t.Error("login body changed")
	}
	for _, k := range []string{"src/auth.ts::formatDate", "src/auth.ts::old"} {
		if _, ok := d.Removed[k]; !ok {
			t.Errorf("Removed missing %s: %v", k, d.Removed)
		}
	}
	for _, k := range []string{"src/auth.ts::formatDay", "src/auth.ts::added"} {
		if _, ok := d.Added[k]; !ok {
			t.Errorf("Added missing %s", k)
		}
	}
	if got := d.Renamed["src/auth.ts::formatDate"]; got != "src/auth.ts::formatDay" {
		t.Errorf("Renamed[formatDate] = %q", got)
	}
	if _, ok := d.Renamed["src/auth.ts::old"]; ok {
		t.Error("old has a different body; must not be a rename")
	}
	if len(d.NewRefs) != 1 || d.NewRefs[0].Callee.Text() != "login" || d.NewRefs[0].ArgCount != 1 {
		t.Errorf("NewRefs = %+v", d.NewRefs)
	}
	if !l.Unsupported["README.md"] || !l.Languages["typescript"] {
		t.Errorf("coverage maps: unsupported=%v languages=%v", l.Unsupported, l.Languages)
	}
	if d.SymbolsCompared == 0 {
		t.Error("SymbolsCompared not counted")
	}
}

func TestLoaderSkipsHugeFiles(t *testing.T) {
	r := gittest.New(t)
	r.Write("src/big.min.js", strings.Repeat("var a=1;", 140000)) // ~1.1 MiB
	head := r.Commit("base")
	l, _ := newLoader(t, r.Dir, false)
	f, err := l.FactsAt(head, "src/big.min.js")
	if err != nil || f != nil {
		t.Fatalf("FactsAt(huge) = %+v, %v; want nil, nil", f, err)
	}
	if !l.Unsupported["src/big.min.js"] {
		t.Error("huge file must be reported as unsupported")
	}
}

func TestLoaderCacheAndRebase(t *testing.T) {
	r := gittest.New(t)
	same := "export function f(): number { return 1; }\n"
	r.Write("a.ts", same)
	r.Write("b.ts", same)
	head := r.Commit("base")
	l, repo := newLoader(t, r.Dir, false)
	fa, err := l.FactsAt(head, "a.ts")
	if err != nil || fa == nil {
		t.Fatal(err)
	}
	entries, _ := filepath.Glob(filepath.Join(repo.CommonDir, "mergelock", "cache", "v1", "typescript@1+ts", "*", "*.gob"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 cache entry, got %v", entries)
	}
	fb, err := l.FactsAt(head, "b.ts") // same blob, served from cache
	if err != nil || fb.Symbols[0].Key != "b.ts::f" || fb.Path != "b.ts" {
		t.Fatalf("cached facts not rebased: %+v", fb)
	}
	l2, _ := newLoader(t, r.Dir, true)
	if f, _ := l2.FactsAt(head, "a.ts"); f == nil || f.Symbols[0].Key != "a.ts::f" {
		t.Fatal("no-cache loader must still parse")
	}
	if missing, err := l.FactsAt(head, "nope.ts"); missing != nil || err != nil {
		t.Fatalf("missing path = %+v, %v", missing, err)
	}
}

// The same blob under .ts and .js is parsed by different grammars, so it
// must not share a cache entry.
func TestLoaderCacheKeyIncludesGrammar(t *testing.T) {
	r := gittest.New(t)
	typed := "export function f(a: string): number { return 1; }\n" // a syntax error for the JS grammar
	r.Write("a.ts", typed)
	r.Write("b.js", typed)
	head := r.Commit("base")
	l, _ := newLoader(t, r.Dir, false)
	if f, err := l.FactsAt(head, "a.ts"); err != nil || f == nil || f.Partial {
		t.Fatalf("a.ts = %+v, %v; want a clean TS parse", f, err)
	}
	f, err := l.FactsAt(head, "b.js")
	if err != nil || f == nil || !f.Partial {
		t.Fatalf("b.js = %+v, %v; want the JS grammar's partial parse, not a.ts's cached facts", f, err)
	}
}

func TestDowngradePartial(t *testing.T) {
	fs := []verdict.Finding{
		{Class: "signature_caller", Confidence: verdict.ConfidenceHigh, CallSites: []verdict.Location{{File: "broken.ts"}}},
		{Class: "textual", Confidence: verdict.ConfidenceCertain, Paths: []string{"broken.ts"}, FromGit: true},
		{Class: "signature_caller", Confidence: verdict.ConfidenceHigh, CallSites: []verdict.Location{{File: "fine.ts"}}},
	}
	downgradePartial(fs, map[string]bool{"broken.ts": true})
	if fs[0].Confidence != verdict.ConfidenceMedium || fs[1].Confidence != verdict.ConfidenceCertain || fs[2].Confidence != verdict.ConfidenceHigh {
		t.Errorf("downgrade wrong: %v %v %v", fs[0].Confidence, fs[1].Confidence, fs[2].Confidence)
	}
}

// panicAdapter claims *.boom files and panics while parsing them.
type panicAdapter struct{}

func (panicAdapter) Name() string                                         { return "boom" }
func (panicAdapter) Version() string                                      { return "1" }
func (panicAdapter) Match(path string) bool                               { return strings.HasSuffix(path, ".boom") }
func (panicAdapter) Parse(string, []byte) (*model.FileFacts, error)       { panic("adapter bug") }
func (panicAdapter) ResolveModule(string, string, lang.FS) (string, bool) { return "", false }
func (panicAdapter) ConstructorName() string                              { return "" }
func (panicAdapter) MaxConfidence() verdict.Confidence                    { return verdict.ConfidenceLow }
func (panicAdapter) SubmoduleSpec(string, string) (string, bool)          { return "", false }

func TestLoaderAdapterPanicIsPartialAndUncached(t *testing.T) {
	r := gittest.New(t)
	r.Write("x.boom", "anything\n")
	head := r.Commit("base")
	repo, err := gitx.Open(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := repo.NewBlobReader()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { blobs.Close() })
	l := NewLoader(repo, blobs, cache.Open(repo.CommonDir, false), lang.NewRegistry(panicAdapter{}))
	f, err := l.FactsAt(head, "x.boom")
	if err != nil || f == nil || !f.Partial || f.Path != "x.boom" || f.Lang != "boom" {
		t.Fatalf("FactsAt(panicking adapter) = %+v, %v; want empty partial facts", f, err)
	}
	if !l.Partial["x.boom"] || !l.Languages["boom"] {
		t.Errorf("coverage maps: partial=%v languages=%v", l.Partial, l.Languages)
	}
	if entries, _ := filepath.Glob(filepath.Join(repo.CommonDir, "mergelock", "cache", "v1", "boom@*")); len(entries) != 0 {
		t.Errorf("recovered result must not be cached, found %v", entries)
	}
}
