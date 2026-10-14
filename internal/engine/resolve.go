package engine

import (
	"strings"

	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

const maxReExportDepth = 5

type resolver struct {
	l   *Loader
	reg *lang.Registry
}

// Matches reports whether ref, read in side's tree, calls sym, and how sure
// we are. See the design doc, rule signature_caller, for the confidence ladder.
func (rc *RuleContext) Matches(side *Side, ref model.Reference, sym model.Symbol) (verdict.Confidence, bool) {
	ad := rc.reg.For(ref.File)
	if ad == nil {
		return "", false
	}
	res := &resolver{l: rc.loader, reg: rc.reg}
	if key, ok := res.resolveRef(side.Commit, ref); ok {
		if key != sym.Key {
			return "", false
		}
		return verdict.MinConfidence(verdict.ConfidenceHigh, ad.MaxConfidence()), true
	}
	c := ref.Callee
	if sym.Kind != model.KindMethod || len(c.Path) == 0 || c.Base == "this" {
		return "", false
	}
	if c.Path[len(c.Path)-1] != sym.Name[strings.LastIndex(sym.Name, ".")+1:] {
		return "", false
	}
	if ref.File != sym.File && !res.importsModule(side.Commit, ref.File, sym.File) {
		return "", false
	}
	return verdict.MinConfidence(verdict.ConfidenceMedium, ad.MaxConfidence()), true
}

func (r *resolver) facts(commit, path string) *model.FileFacts {
	f, err := r.l.FactsAt(commit, path)
	if err != nil {
		return nil
	}
	return f
}

func (r *resolver) module(commit, from, spec string) (string, bool) {
	ad := r.reg.For(from)
	if ad == nil {
		return "", false
	}
	return ad.ResolveModule(from, spec, r.l.FS(commit))
}

func findSymbol(f *model.FileFacts, name string) (model.Symbol, bool) {
	if f != nil {
		for _, s := range f.Symbols {
			if s.Name == name {
				return s, true
			}
		}
	}
	return model.Symbol{}, false
}

// lookupName resolves a bare identifier used in file.
func (r *resolver) lookupName(commit, file, name string) (model.Symbol, bool) {
	f := r.facts(commit, file)
	if f == nil {
		return model.Symbol{}, false
	}
	if s, ok := findSymbol(f, name); ok {
		return s, true
	}
	for _, imp := range f.Imports {
		if imp.Local != name || imp.Imported == "*" {
			continue
		}
		mod, ok := r.module(commit, file, imp.Spec)
		if !ok {
			return model.Symbol{}, false
		}
		return r.findExport(commit, mod, imp.Imported, 0)
	}
	if f.Lang == "python" { // `from m import *` brings names into scope
		for _, re := range f.ReExports {
			if re.Name != "*" {
				continue
			}
			if mod, ok := r.module(commit, file, re.Spec); ok {
				if s, ok := r.findExport(commit, mod, name, 0); ok {
					return s, true
				}
			}
		}
	}
	return model.Symbol{}, false
}

// findExport finds what module mod exports under name, following re-exports.
func (r *resolver) findExport(commit, mod, name string, depth int) (model.Symbol, bool) {
	if depth > maxReExportDepth {
		return model.Symbol{}, false
	}
	f := r.facts(commit, mod)
	if f == nil {
		return model.Symbol{}, false
	}
	if name == "default" {
		if f.Default == "" {
			return model.Symbol{}, false
		}
		name = f.Default
	}
	if s, ok := findSymbol(f, name); ok && s.Exported {
		return s, true
	}
	for _, re := range f.ReExports {
		target := ""
		switch {
		case re.Name == name && re.Imported != "*":
			target = re.Imported
		case re.Name == "*":
			target = name
		default:
			continue
		}
		next, ok := r.module(commit, mod, re.Spec)
		if !ok {
			continue
		}
		if s, ok := r.findExport(commit, next, target, depth+1); ok {
			return s, true
		}
	}
	return model.Symbol{}, false
}

// namespace returns the module bound to a dotted prefix in file: a namespace
// import (`import * as ns`, Python `import a.b`) or a Python submodule import
// (`from pkg import mod`).
func (r *resolver) namespace(commit, file string, f *model.FileFacts, prefix string) (string, bool) {
	for _, imp := range f.Imports {
		if imp.Local != prefix {
			continue
		}
		if imp.Imported == "*" {
			return r.module(commit, file, imp.Spec)
		}
		ad := r.reg.For(file)
		if spec, ok := ad.SubmoduleSpec(imp.Spec, imp.Imported); ok {
			if _, isSymbol := r.lookupName(commit, file, prefix); !isSymbol {
				return r.module(commit, file, spec)
			}
		}
	}
	return "", false
}

func (r *resolver) member(commit string, class model.Symbol, name string) (string, bool) {
	if m, ok := findSymbol(r.facts(commit, class.File), class.Name+"."+name); ok {
		return m.Key, true
	}
	return "", false
}

// callable maps a called symbol to its key; calling a class means calling
// its constructor when one is declared.
func (r *resolver) callable(commit string, s model.Symbol) string {
	if s.Kind != model.KindClass {
		return s.Key
	}
	if ad := r.reg.For(s.File); ad != nil {
		if k, ok := r.member(commit, s, ad.ConstructorName()); ok {
			return k
		}
	}
	return s.Key
}

func enclosingClass(fromKey string) string {
	name := model.ScopeName(fromKey)
	if i := strings.LastIndex(name, "."); i > 0 {
		return name[:i]
	}
	return ""
}

// resolveRef binds ref to exactly one symbol key, or reports false.
func (r *resolver) resolveRef(commit string, ref model.Reference) (string, bool) {
	f := r.facts(commit, ref.File)
	if f == nil {
		return "", false
	}
	c := ref.Callee
	if c.Base == "this" {
		class := enclosingClass(ref.FromKey)
		if class == "" || len(c.Path) != 1 {
			return "", false
		}
		s, ok := findSymbol(f, class)
		if !ok {
			return "", false
		}
		return r.member(commit, s, c.Path[0])
	}
	if len(c.Path) == 0 {
		s, ok := r.lookupName(commit, ref.File, c.Base)
		if !ok {
			return "", false
		}
		return r.callable(commit, s), true
	}
	full := append([]string{c.Base}, c.Path...)
	for i := len(full) - 1; i >= 1; i-- {
		mod, ok := r.namespace(commit, ref.File, f, strings.Join(full[:i], "."))
		if !ok {
			continue
		}
		rest := full[i:]
		s, ok := r.findExport(commit, mod, rest[0], 0)
		if !ok {
			return "", false
		}
		switch {
		case len(rest) == 1:
			return r.callable(commit, s), true
		case len(rest) == 2 && s.Kind == model.KindClass:
			return r.member(commit, s, rest[1])
		}
		return "", false
	}
	if len(c.Path) != 1 {
		return "", false
	}
	recv := c.Base
	if c.ReceiverType != "" {
		recv = c.ReceiverType
	}
	s, ok := r.lookupName(commit, ref.File, recv)
	if !ok || s.Kind != model.KindClass {
		return "", false
	}
	return r.member(commit, s, c.Path[0])
}

// importsModule reports whether file imports target directly or via re-exports.
func (r *resolver) importsModule(commit, file, target string) bool {
	f := r.facts(commit, file)
	if f == nil {
		return false
	}
	var specs []string
	for _, imp := range f.Imports {
		specs = append(specs, imp.Spec)
	}
	for _, re := range f.ReExports {
		specs = append(specs, re.Spec)
	}
	for _, spec := range specs {
		mod, ok := r.module(commit, file, spec)
		if ok && (mod == target || r.reExportsFrom(commit, mod, target, 0)) {
			return true
		}
	}
	return false
}

func (r *resolver) reExportsFrom(commit, mod, target string, depth int) bool {
	if depth >= maxReExportDepth {
		return false
	}
	f := r.facts(commit, mod)
	if f == nil {
		return false
	}
	for _, re := range f.ReExports {
		next, ok := r.module(commit, mod, re.Spec)
		if ok && (next == target || r.reExportsFrom(commit, next, target, depth+1)) {
			return true
		}
	}
	return false
}
