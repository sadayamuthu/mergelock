package typescript

import (
	"fmt"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
)

var identKinds = map[string]bool{
	"identifier": true, "property_identifier": true, "type_identifier": true,
	"shorthand_property_identifier": true, "private_property_identifier": true,
	"shorthand_property_identifier_pattern": true,
}

var literalKinds = map[string]bool{"string_fragment": true, "number": true}

type extractor struct {
	path        string
	src         []byte
	facts       *model.FileFacts
	exportNames map[string]bool // names exported by `export { a, b }`
}

func (x *extractor) text(n *sitter.Node) string { return lang.Text(n, x.src) }

func (x *extractor) key(name string) string { return model.SymbolKey(x.path, name) }

func (x *extractor) add(s model.Symbol) {
	s.Lang = "typescript"
	s.File = x.path
	s.Key = x.key(s.Name)
	x.facts.Symbols = append(x.facts.Symbols, s)
}

// hasToken reports whether n has an anonymous child token tok (e.g. "default").
func hasToken(n *sitter.Node, tok string) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !c.IsNamed() && c.Kind() == tok {
			return true
		}
	}
	return false
}

func childOfKind(n *sitter.Node, kind string) *sitter.Node {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c.Kind() == kind {
			return c
		}
	}
	return nil
}

func stringValue(n *sitter.Node, src []byte) string {
	return strings.Trim(lang.Text(n, src), "\"'`")
}

func (x *extractor) collectDeclarations(root *sitter.Node) {
	for i := uint(0); i < root.NamedChildCount(); i++ {
		n := root.NamedChild(i)
		switch n.Kind() {
		case "import_statement":
			x.importStatement(n)
		case "export_statement":
			x.exportStatement(n)
		default:
			x.declaration(n, false, false)
		}
	}
}

func (x *extractor) exportStatement(n *sitter.Node) {
	isDefault := hasToken(n, "default")
	if decl := n.ChildByFieldName("declaration"); decl != nil {
		x.declaration(decl, true, isDefault)
		return
	}
	if src := n.ChildByFieldName("source"); src != nil {
		x.reExport(n, stringValue(src, x.src))
		return
	}
	if isDefault {
		if val := n.ChildByFieldName("value"); val != nil && val.Kind() == "identifier" {
			x.facts.Default = x.text(val)
			x.exportNames[x.facts.Default] = true
		}
		return
	}
	if clause := childOfKind(n, "export_clause"); clause != nil {
		for i := uint(0); i < clause.NamedChildCount(); i++ {
			if spec := clause.NamedChild(i); spec.Kind() == "export_specifier" {
				x.exportNames[x.text(spec.ChildByFieldName("name"))] = true
			}
		}
	}
}

func isFunctionValue(n *sitter.Node) bool {
	switch n.Kind() {
	case "arrow_function", "function_expression", "function", "generator_function":
		return true
	}
	return false
}

func (x *extractor) declaration(n *sitter.Node, exported, isDefault bool) {
	switch n.Kind() {
	case "function_declaration", "generator_function_declaration":
		name := x.text(n.ChildByFieldName("name"))
		x.addCallable(name, model.KindFunction, n, n, exported)
		if isDefault {
			x.facts.Default = name
		}
	case "class_declaration", "abstract_class_declaration":
		name := x.text(n.ChildByFieldName("name"))
		body := n.ChildByFieldName("body")
		x.add(model.Symbol{Name: name, Kind: model.KindClass, Exported: exported, BodyHash: x.bodyHash(body), Line: lang.Line(n)})
		if isDefault {
			x.facts.Default = name
		}
		if body == nil {
			return
		}
		for i := uint(0); i < body.NamedChildCount(); i++ {
			m := body.NamedChild(i)
			if m.Kind() != "method_definition" {
				continue
			}
			nameNode := m.ChildByFieldName("name")
			if nameNode == nil || (nameNode.Kind() != "property_identifier" && nameNode.Kind() != "private_property_identifier") {
				continue // computed names like [Symbol.iterator]
			}
			x.addCallable(name+"."+x.text(nameNode), model.KindMethod, m, m, exported)
		}
	case "interface_declaration":
		x.add(model.Symbol{Name: x.text(n.ChildByFieldName("name")), Kind: model.KindInterface, Exported: exported,
			BodyHash: x.bodyHash(n.ChildByFieldName("body")), Line: lang.Line(n)})
	case "type_alias_declaration":
		x.add(model.Symbol{Name: x.text(n.ChildByFieldName("name")), Kind: model.KindTypeAlias, Exported: exported,
			BodyHash: x.bodyHash(n.ChildByFieldName("value")), Line: lang.Line(n)})
	case "lexical_declaration", "variable_declaration":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			d := n.NamedChild(i)
			if d.Kind() != "variable_declarator" {
				continue
			}
			nameNode, val := d.ChildByFieldName("name"), d.ChildByFieldName("value")
			if nameNode == nil || nameNode.Kind() != "identifier" || val == nil || !isFunctionValue(val) {
				continue
			}
			x.addCallable(x.text(nameNode), model.KindFunction, val, d, exported)
		}
	}
}

func (x *extractor) bodyHash(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return model.HashStrings(lang.Tokens(n, x.src, nil, nil, false)...)
}

// addCallable records a function or method. fn holds parameters/body; at is
// the node whose start line is reported.
func (x *extractor) addCallable(name string, kind model.Kind, fn, at *sitter.Node, exported bool) {
	var params []model.Param
	var types []string
	paramText := "()"
	if list := fn.ChildByFieldName("parameters"); list != nil {
		params, types = x.params(list)
		paramText = x.text(list)
	} else if single := fn.ChildByFieldName("parameter"); single != nil {
		params, types = []model.Param{{Name: x.text(single)}}, []string{""}
		paramText = "(" + x.text(single) + ")"
	}
	ret := x.text(fn.ChildByFieldName("return_type"))
	short := name[strings.LastIndex(name, ".")+1:]

	parts := make([]string, 0, len(params)+1)
	for i, p := range params {
		parts = append(parts, p.Shape()+":"+model.NormalizeType(types[i]))
	}
	parts = append(parts, "->"+model.NormalizeType(strings.TrimPrefix(strings.TrimSpace(ret), ":")))

	body := fn.ChildByFieldName("body")
	s := model.Symbol{
		Name: name, Kind: kind, Exported: exported,
		Signature: model.Signature{Text: short + paramText + ret, Params: params},
		SigHash:   model.HashStrings(parts...),
		BodyHash:  x.bodyHash(body),
		Line:      lang.Line(at),
	}
	if norm := lang.Tokens(body, x.src, identKinds, literalKinds, true); len(norm) >= model.MinShingleTokens {
		s.Shingles = model.Shingles(norm, model.ShingleSize)
	}
	x.add(s)
}

func patternName(n *sitter.Node, src []byte, i uint) string {
	if n != nil {
		switch n.Kind() {
		case "identifier", "this":
			return lang.Text(n, src)
		case "rest_pattern":
			if n.NamedChildCount() > 0 {
				return patternName(n.NamedChild(0), src, i)
			}
		}
	}
	return fmt.Sprintf("$%d", i)
}

func (x *extractor) params(list *sitter.Node) ([]model.Param, []string) {
	var ps []model.Param
	var types []string
	for i := uint(0); i < list.NamedChildCount(); i++ {
		n := list.NamedChild(i)
		var p model.Param
		typ := ""
		switch n.Kind() {
		case "required_parameter", "optional_parameter":
			pat := n.ChildByFieldName("pattern")
			p.Name = patternName(pat, x.src, i)
			if p.Name == "this" {
				continue // TS `this` parameter is not passed by callers
			}
			p.Optional = n.Kind() == "optional_parameter" || n.ChildByFieldName("value") != nil
			p.Rest = pat != nil && pat.Kind() == "rest_pattern"
			typ = strings.TrimPrefix(strings.TrimSpace(x.text(n.ChildByFieldName("type"))), ":")
		case "identifier":
			p.Name = x.text(n)
		case "assignment_pattern":
			p.Name = patternName(n.ChildByFieldName("left"), x.src, i)
			p.Optional = true
		case "rest_pattern":
			p.Name = patternName(n, x.src, i)
			p.Rest = true
		case "object_pattern", "array_pattern":
			p.Name = fmt.Sprintf("$%d", i)
		default:
			continue // comments, decorators
		}
		ps = append(ps, p)
		types = append(types, typ)
	}
	return ps, types
}

func (x *extractor) applyLocalExports() {
	for i := range x.facts.Symbols {
		s := &x.facts.Symbols[i]
		owner := s.Name
		if dot := strings.Index(owner, "."); dot >= 0 {
			owner = owner[:dot]
		}
		if x.exportNames[owner] {
			s.Exported = true
		}
	}
}
