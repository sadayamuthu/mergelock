package python

import (
	"maps"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
)

func (x *extractor) walk(n *sitter.Node, enc, class string, hints map[string]string, top bool) {
	switch n.Kind() {
	case "call":
		x.reference(n, enc, hints)
	case "function_definition":
		if top {
			name := x.text(n.ChildByFieldName("name"))
			if class != "" {
				name = class + "." + name
			}
			enc = model.SymbolKey(x.path, name)
		}
		hints, top = withHints(hints, n, x.src), false
	case "class_definition":
		if top && class == "" {
			class = x.text(n.ChildByFieldName("name"))
		} else {
			top = false
		}
	case "lambda":
		top = false
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		x.walk(n.NamedChild(i), enc, class, hints, top)
	}
}

func (x *extractor) reference(n *sitter.Node, enc string, hints map[string]string) {
	base, path, ok := flatten(n.ChildByFieldName("function"), x.src)
	if !ok {
		return
	}
	if base == "self" {
		base = "this"
	}
	count, spread, kws := countArgs(n.ChildByFieldName("arguments"), x.src)
	c := model.CalleeExpr{Base: base, Path: path}
	if base != "this" && len(path) > 0 {
		c.ReceiverType = hints[base]
	}
	x.facts.References = append(x.facts.References, model.Reference{
		FromKey: enc, Callee: c, ArgCount: count, Spread: spread, Keywords: kws,
		File: x.path, Line: lang.Line(n), Text: lang.Snippet(n, x.src),
	})
}

func flatten(n *sitter.Node, src []byte) (string, []string, bool) {
	if n == nil {
		return "", nil, false
	}
	switch n.Kind() {
	case "identifier":
		return lang.Text(n, src), nil, true
	case "attribute":
		attr := n.ChildByFieldName("attribute")
		base, path, ok := flatten(n.ChildByFieldName("object"), src)
		if !ok || attr == nil {
			return "", nil, false
		}
		return base, append(append([]string(nil), path...), lang.Text(attr, src)), true
	}
	return "", nil, false
}

func countArgs(args *sitter.Node, src []byte) (int, bool, []string) {
	if args == nil {
		return 0, false, nil
	}
	if args.Kind() != "argument_list" {
		return 1, false, nil // f(x for x in xs)
	}
	count, spread := 0, false
	var kws []string
	for i := uint(0); i < args.NamedChildCount(); i++ {
		a := args.NamedChild(i)
		switch a.Kind() {
		case "comment":
		case "keyword_argument":
			kws = append(kws, lang.Text(a.ChildByFieldName("name"), src))
		case "list_splat", "dictionary_splat":
			spread = true
		default:
			count++
		}
	}
	return count, spread, kws
}

func withHints(parent map[string]string, fn *sitter.Node, src []byte) map[string]string {
	h := make(map[string]string, len(parent))
	maps.Copy(h, parent)
	if ps := fn.ChildByFieldName("parameters"); ps != nil {
		for i := uint(0); i < ps.NamedChildCount(); i++ {
			p := ps.NamedChild(i)
			var name *sitter.Node
			switch p.Kind() {
			case "typed_parameter":
				name = p.NamedChild(0)
			case "typed_default_parameter":
				name = p.ChildByFieldName("name")
			default:
				continue
			}
			if name == nil || name.Kind() != "identifier" {
				continue
			}
			if t := typeName(p.ChildByFieldName("type"), src); t != "" {
				h[lang.Text(name, src)] = t
			}
		}
	}
	if body := fn.ChildByFieldName("body"); body != nil {
		assignmentHints(body, src, h)
	}
	return h
}

func assignmentHints(n *sitter.Node, src []byte, h map[string]string) {
	switch n.Kind() {
	case "function_definition", "class_definition":
		return // nested scopes collect their own hints
	case "assignment":
		left := n.ChildByFieldName("left")
		if left != nil && left.Kind() == "identifier" {
			if t := typeName(n.ChildByFieldName("type"), src); t != "" {
				h[lang.Text(left, src)] = t
			} else if right := n.ChildByFieldName("right"); right != nil && right.Kind() == "call" {
				if fn := right.ChildByFieldName("function"); fn != nil && fn.Kind() == "identifier" {
					h[lang.Text(left, src)] = lang.Text(fn, src)
				}
			}
		}
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		assignmentHints(n.NamedChild(i), src, h)
	}
}

func typeName(t *sitter.Node, src []byte) string {
	if t == nil {
		return ""
	}
	if t.Kind() == "type" && t.NamedChildCount() == 1 {
		t = t.NamedChild(0)
	}
	if t.Kind() == "identifier" {
		return lang.Text(t, src)
	}
	return ""
}
