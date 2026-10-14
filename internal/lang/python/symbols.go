package python

import (
	"slices"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
)

var identKinds = map[string]bool{"identifier": true}
var literalKinds = map[string]bool{"string_content": true, "integer": true, "float": true}

type extractor struct {
	path   string
	src    []byte
	isInit bool
	facts  *model.FileFacts
}

func (x *extractor) text(n *sitter.Node) string { return lang.Text(n, x.src) }

func (x *extractor) add(s model.Symbol) {
	s.Lang = "python"
	s.File = x.path
	s.Key = model.SymbolKey(x.path, s.Name)
	s.Exported = true
	x.facts.Symbols = append(x.facts.Symbols, s)
}

func (x *extractor) module(root *sitter.Node) {
	for i := uint(0); i < root.NamedChildCount(); i++ {
		n := root.NamedChild(i)
		switch n.Kind() {
		case "import_statement":
			x.importStatement(n)
		case "import_from_statement":
			x.importFrom(n)
		case "function_definition", "class_definition", "decorated_definition":
			x.definition(n, "")
		}
	}
}

func unwrap(n *sitter.Node, src []byte) (*sitter.Node, []string) {
	if n.Kind() != "decorated_definition" {
		return n, nil
	}
	var decs []string
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c.Kind() == "decorator" {
			decs = append(decs, strings.TrimPrefix(strings.TrimSpace(lang.Text(c, src)), "@"))
		}
	}
	return n.ChildByFieldName("definition"), decs
}

func (x *extractor) definition(n *sitter.Node, class string) {
	def, decs := unwrap(n, x.src)
	if def == nil {
		return
	}
	name := x.text(def.ChildByFieldName("name"))
	switch def.Kind() {
	case "function_definition":
		if class == "" {
			x.addCallable(name, model.KindFunction, def, false)
			return
		}
		x.addCallable(class+"."+name, model.KindMethod, def, !slices.Contains(decs, "staticmethod"))
	case "class_definition":
		if class != "" {
			return // nested classes are not tracked
		}
		body := def.ChildByFieldName("body")
		x.add(model.Symbol{Name: name, Kind: model.KindClass, BodyHash: lang.BodyHash(body, x.src), Line: lang.Line(def)})
		if body == nil {
			return
		}
		for i := uint(0); i < body.NamedChildCount(); i++ {
			switch c := body.NamedChild(i); c.Kind() {
			case "function_definition", "decorated_definition":
				x.definition(c, name)
			}
		}
	}
}

func (x *extractor) addCallable(name string, kind model.Kind, def *sitter.Node, dropFirst bool) {
	list := def.ChildByFieldName("parameters")
	params, types := x.params(list)
	if dropFirst && len(params) > 0 && params[0].Shape() == "req" {
		params, types = params[1:], types[1:]
	}
	ret := def.ChildByFieldName("return_type")
	short := name[strings.LastIndex(name, ".")+1:]
	text := short + x.text(list)
	if ret != nil {
		text += " -> " + x.text(ret)
	}
	parts := make([]string, 0, len(params)+1)
	for i, p := range params {
		// Names are part of a Python call contract (keyword arguments).
		parts = append(parts, p.Name+":"+p.Shape()+":"+model.NormalizeType(types[i]))
	}
	parts = append(parts, "->"+model.NormalizeType(x.text(ret)))
	body := def.ChildByFieldName("body")
	s := model.Symbol{
		Name: name, Kind: kind,
		Signature: model.Signature{Text: text, Params: params},
		SigHash:   model.HashStrings(parts...),
		BodyHash:  lang.BodyHash(body, x.src),
		Shingles:  lang.BodyShingles(body, x.src, identKinds, literalKinds),
		Line:      lang.Line(def),
	}
	x.add(s)
}

func (x *extractor) splatName(n *sitter.Node) string {
	if n.NamedChildCount() > 0 {
		return x.text(n.NamedChild(0))
	}
	return ""
}

func (x *extractor) params(list *sitter.Node) ([]model.Param, []string) {
	if list == nil {
		return nil, nil
	}
	var ps []model.Param
	var types []string
	kwOnly := false
	for i := uint(0); i < list.NamedChildCount(); i++ {
		n := list.NamedChild(i)
		var p model.Param
		typ := ""
		switch n.Kind() {
		case "identifier":
			p.Name = x.text(n)
		case "typed_parameter":
			typ = x.text(n.ChildByFieldName("type"))
			inner := n.NamedChild(0)
			if inner == nil {
				continue
			}
			switch inner.Kind() {
			case "list_splat_pattern":
				p.Name, p.Rest = x.splatName(inner), true
			case "dictionary_splat_pattern":
				p.Name, p.KwRest = x.splatName(inner), true
			default:
				p.Name = x.text(inner)
			}
		case "default_parameter", "typed_default_parameter":
			p.Name, p.Optional = x.text(n.ChildByFieldName("name")), true
			typ = x.text(n.ChildByFieldName("type"))
		case "list_splat_pattern":
			p.Name, p.Rest = x.splatName(n), true
		case "dictionary_splat_pattern":
			p.Name, p.KwRest = x.splatName(n), true
		case "keyword_separator":
			kwOnly = true
			continue
		default:
			continue // positional_separator, comments
		}
		if kwOnly && !p.Rest && !p.KwRest {
			p.KwOnly = true
		}
		if p.Rest {
			kwOnly = true
		}
		ps = append(ps, p)
		types = append(types, typ)
	}
	return ps, types
}

func (x *extractor) importStatement(n *sitter.Node) {
	for i := uint(0); i < n.ChildCount(); i++ {
		if n.FieldNameForChild(uint32(i)) != "name" {
			continue
		}
		switch c := n.Child(i); c.Kind() {
		case "dotted_name":
			mod := x.text(c)
			x.facts.Imports = append(x.facts.Imports, model.Import{Local: mod, Imported: "*", Spec: mod})
		case "aliased_import":
			x.facts.Imports = append(x.facts.Imports, model.Import{
				Local: x.text(c.ChildByFieldName("alias")), Imported: "*", Spec: x.text(c.ChildByFieldName("name")),
			})
		}
	}
}

func (x *extractor) importFrom(n *sitter.Node) {
	spec := x.text(n.ChildByFieldName("module_name"))
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Kind() == "wildcard_import" {
			x.facts.Imports = append(x.facts.Imports, model.Import{Local: "", Imported: "*", Spec: spec})
			x.facts.ReExports = append(x.facts.ReExports, model.ReExport{Name: "*", Imported: "*", Spec: spec})
			continue
		}
		if n.FieldNameForChild(uint32(i)) != "name" {
			continue
		}
		var name, local string
		switch c.Kind() {
		case "dotted_name":
			name = x.text(c)
			local = name
		case "aliased_import":
			name, local = x.text(c.ChildByFieldName("name")), x.text(c.ChildByFieldName("alias"))
		default:
			continue
		}
		x.facts.Imports = append(x.facts.Imports, model.Import{Local: local, Imported: name, Spec: spec})
		if x.isInit {
			x.facts.ReExports = append(x.facts.ReExports, model.ReExport{Name: local, Imported: name, Spec: spec})
		}
	}
}
