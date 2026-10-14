// Package python extracts symbols, imports and call references from Python
// using tree-sitter.
package python

import (
	"fmt"
	"path"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tspy "github.com/tree-sitter/tree-sitter-python/bindings/go"

	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

var langPy = sitter.NewLanguage(tspy.Language())

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (*Adapter) Name() string                      { return "python" }
func (*Adapter) Version() string                   { return "1" }
func (*Adapter) ConstructorName() string           { return "__init__" }
func (*Adapter) MaxConfidence() verdict.Confidence { return verdict.ConfidenceMedium }
func (*Adapter) Match(p string) bool               { return path.Ext(p) == ".py" }

func (*Adapter) SubmoduleSpec(spec, name string) (string, bool) {
	if strings.HasSuffix(spec, ".") {
		return spec + name, true
	}
	return spec + "." + name, true
}

func (a *Adapter) Parse(p string, src []byte) (*model.FileFacts, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(langPy); err != nil {
		return nil, err
	}
	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("python: parse %s failed", p)
	}
	defer tree.Close()
	root := tree.RootNode()
	x := &extractor{
		path:   p,
		src:    src,
		isInit: path.Base(p) == "__init__.py",
		facts:  &model.FileFacts{Path: p, Lang: "python", Partial: root.HasError()},
	}
	x.module(root)
	x.walk(root, model.SymbolKey(p, model.ModuleScope), "", nil, true)
	return x.facts, nil
}
