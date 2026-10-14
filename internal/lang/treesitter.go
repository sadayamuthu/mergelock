package lang

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/sadayamuthu/mergelock/internal/model"
)

// Text returns n's source text, or "" for a nil node.
func Text(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return n.Utf8Text(src)
}

// Line returns the 1-based line where n starts.
func Line(n *sitter.Node) int {
	return int(n.StartPosition().Row) + 1
}

// Tokens returns n's leaf tokens in source order, skipping comments. With
// normalize, identifier kinds become "$id" and literal kinds "$lit".
func Tokens(n *sitter.Node, src []byte, idents, literals map[string]bool, normalize bool) []string {
	var out []string
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Kind() == "comment" {
			return
		}
		if n.ChildCount() == 0 {
			switch {
			case normalize && idents[n.Kind()]:
				out = append(out, "$id")
			case normalize && literals[n.Kind()]:
				out = append(out, "$lit")
			default:
				out = append(out, n.Utf8Text(src))
			}
			return
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	if n != nil {
		walk(n)
	}
	return out
}

// Snippet returns n's text collapsed onto one line, at most 120 bytes.
func Snippet(n *sitter.Node, src []byte) string {
	s := strings.Join(strings.Fields(Text(n, src)), " ")
	if len(s) > 120 {
		s = s[:117] + "..."
	}
	return s
}

// BodyHash hashes n's raw tokens, so comments and whitespace do not count.
// It returns "" for a nil node.
func BodyHash(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return model.HashStrings(Tokens(n, src, nil, nil, false)...)
}

// BodyShingles returns the shingles of n's normalized tokens, or nil when the
// body is too short (fewer than model.MinShingleTokens tokens) to compare.
func BodyShingles(n *sitter.Node, src []byte, idents, literals map[string]bool) []uint64 {
	norm := Tokens(n, src, idents, literals, true)
	if len(norm) < model.MinShingleTokens {
		return nil
	}
	return model.Shingles(norm, model.ShingleSize)
}
