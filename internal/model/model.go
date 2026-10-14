// Package model holds the language-neutral facts extracted from source files.
// It imports nothing from this module.
package model

import (
	"fmt"
	"sort"
	"strings"
)

type Kind string

const (
	KindFunction  Kind = "function"
	KindMethod    Kind = "method"
	KindClass     Kind = "class"
	KindInterface Kind = "interface"
	KindTypeAlias Kind = "type_alias"
)

type Param struct {
	Name     string
	Optional bool // TS `?` or a default value (TS/Python)
	Rest     bool // TS `...x`, Python `*args`
	KwRest   bool // Python `**kwargs`
	KwOnly   bool // Python parameters after `*` or `*args`
}

// Shape is the parameter's calling convention, independent of its name.
func (p Param) Shape() string {
	switch {
	case p.Rest:
		return "rest"
	case p.KwRest:
		return "kwrest"
	case p.KwOnly && p.Optional:
		return "kwopt"
	case p.KwOnly:
		return "kw"
	case p.Optional:
		return "opt"
	}
	return "req"
}

type Signature struct {
	Text   string // as written, for verdict output
	Params []Param
}

type Symbol struct {
	Lang      string
	File      string // repo-relative path
	Name      string // display name, e.g. "AuthService.login"
	Key       string // unique: File + "::" + Name
	Kind      Kind
	Exported  bool
	Signature Signature
	SigHash   string
	BodyHash  string
	Shingles  []uint64 // only for functions/methods with >= MinShingleTokens body tokens
	Line      int
}

// ModuleScope names code that is not inside any tracked symbol.
const ModuleScope = "<module>"

func SymbolKey(file, name string) string { return file + "::" + name }

// ScopeName returns the name part of a symbol key.
func ScopeName(key string) string {
	if i := strings.LastIndex(key, "::"); i >= 0 {
		return key[i+2:]
	}
	return key
}

type Import struct {
	Local    string // local binding; "" for a Python star import
	Imported string // exported name; "*" for namespace/module imports, "default" for default imports
	Spec     string // module specifier as written
}

type ReExport struct {
	Name     string // name exported by this module; "*" for export-all
	Imported string // name in the source module; "*" for export-all
	Spec     string
}

type CalleeExpr struct {
	Base         string   // leftmost identifier; "this" for this/self
	Path         []string // member chain after Base
	New          bool     // constructor call (`new X()`)
	ReceiverType string   // type hint for Base when known locally
}

func (c CalleeExpr) Text() string {
	s := strings.Join(append([]string{c.Base}, c.Path...), ".")
	if c.New {
		return "new " + s
	}
	return s
}

type Reference struct {
	FromKey  string // enclosing symbol key, or SymbolKey(file, ModuleScope)
	Callee   CalleeExpr
	ArgCount int // positional arguments, excluding spreads and keywords
	Spread   bool
	Keywords []string
	File     string
	Line     int
	Text     string
}

// Identity is what makes two references "the same call" across revisions:
// enclosing symbol name, callee, argument shape. File path is excluded so
// that renamed files do not produce spurious new references.
func (r Reference) Identity() string {
	kws := append([]string(nil), r.Keywords...)
	sort.Strings(kws)
	return fmt.Sprintf("%s|%s|%d|%t|%s", ScopeName(r.FromKey), r.Callee.Text(), r.ArgCount, r.Spread, strings.Join(kws, ","))
}

type FileFacts struct {
	Path       string
	Lang       string
	Partial    bool   // the parser reported syntax errors
	Default    string // name of the default-exported symbol (TS), if any
	Symbols    []Symbol
	Imports    []Import
	ReExports  []ReExport
	References []Reference
}

// Rebase rewrites every path-derived field for a new path. Cached facts are
// keyed by blob, and the same blob can live at different paths.
func (f *FileFacts) Rebase(path string) {
	f.Path = path
	for i := range f.Symbols {
		f.Symbols[i].File = path
		f.Symbols[i].Key = SymbolKey(path, f.Symbols[i].Name)
	}
	for i := range f.References {
		f.References[i].File = path
		f.References[i].FromKey = SymbolKey(path, ScopeName(f.References[i].FromKey))
	}
}
