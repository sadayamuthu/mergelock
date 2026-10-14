package typescript

import (
	"maps"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
)

func (x *extractor) importStatement(n *sitter.Node) {
	src := n.ChildByFieldName("source")
	clause := childOfKind(n, "import_clause")
	if src == nil || clause == nil {
		return
	}
	spec := stringValue(src, x.src)
	for i := uint(0); i < clause.NamedChildCount(); i++ {
		c := clause.NamedChild(i)
		switch c.Kind() {
		case "identifier":
			x.facts.Imports = append(x.facts.Imports, model.Import{Local: x.text(c), Imported: "default", Spec: spec})
		case "namespace_import":
			if id := childOfKind(c, "identifier"); id != nil {
				x.facts.Imports = append(x.facts.Imports, model.Import{Local: x.text(id), Imported: "*", Spec: spec})
			}
		case "named_imports":
			for j := uint(0); j < c.NamedChildCount(); j++ {
				s := c.NamedChild(j)
				if s.Kind() != "import_specifier" {
					continue
				}
				name := x.text(s.ChildByFieldName("name"))
				local := name
				if alias := s.ChildByFieldName("alias"); alias != nil {
					local = x.text(alias)
				}
				x.facts.Imports = append(x.facts.Imports, model.Import{Local: local, Imported: name, Spec: spec})
			}
		}
	}
}

func (x *extractor) reExport(n *sitter.Node, spec string) {
	clause := childOfKind(n, "export_clause")
	if clause == nil {
		if hasToken(n, "*") && childOfKind(n, "namespace_export") == nil {
			x.facts.ReExports = append(x.facts.ReExports, model.ReExport{Name: "*", Imported: "*", Spec: spec})
		}
		return
	}
	for i := uint(0); i < clause.NamedChildCount(); i++ {
		s := clause.NamedChild(i)
		if s.Kind() != "export_specifier" {
			continue
		}
		name := x.text(s.ChildByFieldName("name"))
		exported := name
		if alias := s.ChildByFieldName("alias"); alias != nil {
			exported = x.text(alias)
		}
		x.facts.ReExports = append(x.facts.ReExports, model.ReExport{Name: exported, Imported: name, Spec: spec})
	}
}

// walk visits every node, tracking the enclosing symbol key (enc), the
// enclosing top-level class, local type hints and whether we are still at
// declaration level (top).
func (x *extractor) walk(n *sitter.Node, enc, class string, hints map[string]string, top bool) {
	switch n.Kind() {
	case "call_expression", "new_expression":
		x.reference(n, enc, hints)
	case "function_declaration", "generator_function_declaration":
		if top {
			enc = x.key(x.text(n.ChildByFieldName("name")))
		}
		hints, top = withHints(hints, n, x.src), false
	case "class_declaration", "abstract_class_declaration":
		class = ""
		if top {
			class = x.text(n.ChildByFieldName("name"))
		}
	case "method_definition":
		if top && class != "" {
			enc = x.key(class + "." + x.text(n.ChildByFieldName("name")))
		}
		hints, top = withHints(hints, n, x.src), false
	case "variable_declarator":
		if v := n.ChildByFieldName("value"); top && v != nil && isFunctionValue(v) {
			enc = x.key(x.text(n.ChildByFieldName("name")))
		}
	case "arrow_function", "function_expression", "function", "generator_function":
		hints, top = withHints(hints, n, x.src), false
	case "statement_block":
		top = false
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		x.walk(n.NamedChild(i), enc, class, hints, top)
	}
}

func (x *extractor) reference(n *sitter.Node, enc string, hints map[string]string) {
	isNew := n.Kind() == "new_expression"
	field := "function"
	if isNew {
		field = "constructor"
	}
	base, path, ok := flatten(n.ChildByFieldName(field), x.src)
	if !ok {
		return
	}
	count, spread := countArgs(n.ChildByFieldName("arguments"))
	c := model.CalleeExpr{Base: base, Path: path, New: isNew}
	if base != "this" && len(path) > 0 {
		c.ReceiverType = hints[base]
	}
	x.facts.References = append(x.facts.References, model.Reference{
		FromKey: enc, Callee: c, ArgCount: count, Spread: spread,
		File: x.path, Line: lang.Line(n), Text: lang.Snippet(n, x.src),
	})
}

// flatten turns `a.b.c` into ("a", ["b", "c"]). Anything that is not an
// identifier/this rooted member chain is not resolvable and returns false.
func flatten(n *sitter.Node, src []byte) (string, []string, bool) {
	if n == nil {
		return "", nil, false
	}
	switch n.Kind() {
	case "identifier":
		return lang.Text(n, src), nil, true
	case "this":
		return "this", nil, true
	case "member_expression":
		prop := n.ChildByFieldName("property")
		base, path, ok := flatten(n.ChildByFieldName("object"), src)
		if !ok || prop == nil {
			return "", nil, false
		}
		out := append(append([]string(nil), path...), lang.Text(prop, src))
		return base, out, true
	}
	return "", nil, false
}

func countArgs(args *sitter.Node) (int, bool) {
	if args == nil {
		return 0, false
	}
	count, spread := 0, false
	for i := uint(0); i < args.NamedChildCount(); i++ {
		switch args.NamedChild(i).Kind() {
		case "comment":
		case "spread_element":
			spread = true
		default:
			count++
		}
	}
	return count, spread
}

// withHints returns parent plus type hints from fn's parameters and from
// variable declarations anywhere in its body.
func withHints(parent map[string]string, fn *sitter.Node, src []byte) map[string]string {
	h := make(map[string]string, len(parent))
	maps.Copy(h, parent)
	if ps := fn.ChildByFieldName("parameters"); ps != nil {
		for i := uint(0); i < ps.NamedChildCount(); i++ {
			p := ps.NamedChild(i)
			if p.Kind() != "required_parameter" && p.Kind() != "optional_parameter" {
				continue
			}
			pat := p.ChildByFieldName("pattern")
			if pat == nil || pat.Kind() != "identifier" {
				continue
			}
			if t := typeName(p.ChildByFieldName("type"), src); t != "" {
				h[lang.Text(pat, src)] = t
			}
		}
	}
	if body := fn.ChildByFieldName("body"); body != nil {
		declaratorHints(body, src, h)
	}
	return h
}

func declaratorHints(n *sitter.Node, src []byte, h map[string]string) {
	if n.Kind() == "variable_declarator" {
		name := n.ChildByFieldName("name")
		if name != nil && name.Kind() == "identifier" {
			if t := typeName(n.ChildByFieldName("type"), src); t != "" {
				h[lang.Text(name, src)] = t
			} else if v := n.ChildByFieldName("value"); v != nil && v.Kind() == "new_expression" {
				if c := v.ChildByFieldName("constructor"); c != nil && c.Kind() == "identifier" {
					h[lang.Text(name, src)] = lang.Text(c, src)
				}
			}
		}
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		declaratorHints(n.NamedChild(i), src, h)
	}
}

// typeName returns the class name of a simple type annotation (`: X` or
// `: X<T>`), or "" for anything else.
func typeName(ann *sitter.Node, src []byte) string {
	if ann == nil {
		return ""
	}
	t := ann
	if t.Kind() == "type_annotation" && t.NamedChildCount() > 0 {
		t = t.NamedChild(0)
	}
	switch t.Kind() {
	case "type_identifier":
		return lang.Text(t, src)
	case "generic_type":
		return lang.Text(t.ChildByFieldName("name"), src)
	}
	return ""
}
