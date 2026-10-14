package model

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"sort"
	"strings"
	"unicode"
)

const (
	ShingleSize      = 5
	MinShingleTokens = 30
)

// HashStrings hashes an ordered list of strings; part boundaries matter.
func HashStrings(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// Shingles returns the sorted, unique hashes of every k-token window.
func Shingles(tokens []string, k int) []uint64 {
	if len(tokens) < k {
		return nil
	}
	set := map[uint64]bool{}
	for i := 0; i+k <= len(tokens); i++ {
		h := fnv.New64a()
		for _, t := range tokens[i : i+k] {
			h.Write([]byte(t))
			h.Write([]byte{0})
		}
		set[h.Sum64()] = true
	}
	out := make([]uint64, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Jaccard is |a∩b| / |a∪b| for sorted unique sets.
func Jaccard(a, b []uint64) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter, i, j := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			inter++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// NormalizeType strips whitespace and generic arguments from a type annotation.
func NormalizeType(s string) string {
	var b strings.Builder
	depth := 0
	for i, r := range s {
		switch {
		case unicode.IsSpace(r):
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case r == '>' && i > 0 && s[i-1] == '=':
			b.WriteRune(r) // arrow `=>`
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}
