// Package typescript extracts symbols, imports and call references from
// TypeScript and JavaScript using tree-sitter.
package typescript

import (
	"fmt"
	"path"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

var (
	langTS  = sitter.NewLanguage(tsts.LanguageTypescript())
	langTSX = sitter.NewLanguage(tsts.LanguageTSX())
	langJS  = sitter.NewLanguage(tsjs.Language())
)

var extensions = map[string]bool{
	".ts": true, ".tsx": true, ".mts": true, ".cts": true,
	".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
}

type Adapter struct {
	mu        sync.Mutex
	tsconfigs map[string]*tsconfig // parsed tsconfig.json keyed by its content
}

func New() *Adapter { return &Adapter{tsconfigs: map[string]*tsconfig{}} }

func (*Adapter) Name() string                                { return "typescript" }
func (*Adapter) Version() string                             { return "1" }
func (*Adapter) ConstructorName() string                     { return "constructor" }
func (*Adapter) MaxConfidence() verdict.Confidence           { return verdict.ConfidenceHigh }
func (*Adapter) SubmoduleSpec(string, string) (string, bool) { return "", false }
func (*Adapter) Match(p string) bool                         { return extensions[path.Ext(p)] }

func languageFor(p string) *sitter.Language {
	switch path.Ext(p) {
	case ".tsx":
		return langTSX
	case ".ts", ".mts", ".cts":
		return langTS
	}
	return langJS
}

func (a *Adapter) Parse(p string, src []byte) (*model.FileFacts, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(languageFor(p)); err != nil {
		return nil, err
	}
	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("typescript: parse %s failed", p)
	}
	defer tree.Close()
	root := tree.RootNode()
	x := &extractor{
		path:        p,
		src:         src,
		facts:       &model.FileFacts{Path: p, Lang: "typescript", Partial: root.HasError()},
		exportNames: map[string]bool{},
	}
	x.collectDeclarations(root)
	x.walk(root, model.SymbolKey(p, model.ModuleScope), "", nil, true)
	x.applyLocalExports()
	return x.facts, nil
}

// ResolveModule is implemented in Task 10.
func (a *Adapter) ResolveModule(fromFile, spec string, fs lang.FS) (string, bool) {
	return "", false
}

// tsconfig is filled in by Task 10.
type tsconfig struct{}
