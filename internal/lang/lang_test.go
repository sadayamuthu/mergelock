package lang_test

import (
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
