package lang_test

import (
	"slices"
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"

	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

type fake struct{ ext string }

func (f fake) Name() string                                       { return "fake" + f.ext }
func (fake) Version() string                                      { return "1" }
func (f fake) Match(p string) bool                                { return strings.HasSuffix(p, f.ext) }
func (fake) Parse(string, []byte) (*model.FileFacts, error)       { return nil, nil }
func (fake) ResolveModule(string, string, lang.FS) (string, bool) { return "", false }
func (fake) ConstructorName() string                              { return "init" }
func (fake) MaxConfidence() verdict.Confidence                    { return verdict.ConfidenceHigh }
func (fake) SubmoduleSpec(string, string) (string, bool)          { return "", false }

func TestRegistry(t *testing.T) {
	r := lang.NewRegistry(fake{".ts"}, fake{".py"})
	if a := r.For("src/x.py"); a == nil || a.Name() != "fake.py" {
		t.Errorf("For(.py) = %v", a)
	}
	if a := r.For("README.md"); a != nil {
		t.Errorf("For(.md) = %v, want nil", a)
	}
	if len(r.All()) != 2 {
		t.Error("All")
	}
}

func TestTokens(t *testing.T) {
	src := []byte("function f(a) { // note\n  return a + 1; }")
	p := sitter.NewParser()
	defer p.Close()
	p.SetLanguage(sitter.NewLanguage(tsjs.Language()))
	tree := p.Parse(src, nil)
	defer tree.Close()
	fn := tree.RootNode().NamedChild(0)
	body := fn.ChildByFieldName("body")
	raw := lang.Tokens(body, src, nil, nil, false)
	if strings.Join(raw, " ") != "{ return a + 1 ; }" {
		t.Errorf("raw tokens = %q", raw)
	}
	norm := lang.Tokens(body, src, map[string]bool{"identifier": true}, map[string]bool{"number": true}, true)
	if strings.Join(norm, " ") != "{ return $id + $lit ; }" {
		t.Errorf("normalized tokens = %q", norm)
	}
	if lang.Line(body) != 1 || lang.Text(nil, src) != "" {
		t.Error("Line/Text")
	}
	if got := lang.Snippet(fn, src); strings.Contains(got, "\n") || !strings.HasPrefix(got, "function f(a)") {
		t.Errorf("Snippet = %q", got)
	}
}

func TestBodyHashAndShingles(t *testing.T) {
	parse := func(src string) (*sitter.Node, []byte, func()) {
		p := sitter.NewParser()
		p.SetLanguage(sitter.NewLanguage(tsjs.Language()))
		tree := p.Parse([]byte(src), nil)
		body := tree.RootNode().NamedChild(0).ChildByFieldName("body")
		return body, []byte(src), func() { tree.Close(); p.Close() }
	}
	ids, lits := map[string]bool{"identifier": true}, map[string]bool{"number": true}

	a, srcA, closeA := parse("function f(a) { return a + 1; }")
	defer closeA()
	b, srcB, closeB := parse("function f(a) { // note\n  return a + 1; }")
	defer closeB()
	if lang.BodyHash(nil, srcA) != "" || lang.BodyHash(a, srcA) == "" {
		t.Error("BodyHash nil/non-nil")
	}
	if lang.BodyHash(a, srcA) != lang.BodyHash(b, srcB) {
		t.Error("BodyHash must ignore comments and whitespace")
	}
	if lang.BodyShingles(a, srcA, ids, lits) != nil || lang.BodyShingles(nil, srcA, ids, lits) != nil {
		t.Error("short or nil bodies must have no shingles")
	}
	long := "function f(a) { const b = a + 1; const c = b * 2; const d = c - 3; return a + b + c + d; }"
	l, srcL, closeL := parse(long)
	defer closeL()
	norm := lang.Tokens(l, srcL, ids, lits, true)
	want := model.Shingles(norm, model.ShingleSize)
	if got := lang.BodyShingles(l, srcL, ids, lits); len(norm) < model.MinShingleTokens || !slices.Equal(got, want) {
		t.Errorf("BodyShingles = %v, want %v (tokens %d)", got, want, len(norm))
	}
}
