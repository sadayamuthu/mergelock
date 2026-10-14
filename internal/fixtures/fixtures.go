// Package fixtures runs branch-pair fixtures with known ground truth through
// the real engine. It is the v0.1 quality gate.
package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/gittest"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

type ExpectedFinding struct {
	Class         string             `json:"class"`
	Kind          string             `json:"kind,omitempty"`
	Symbol        string             `json:"symbol,omitempty"`
	RenamedTo     string             `json:"renamed_to,omitempty"`
	MinConfidence verdict.Confidence `json:"min_confidence,omitempty"`
	MaxConfidence verdict.Confidence `json:"max_confidence,omitempty"`
}

type Expected struct {
	Result   verdict.Result    `json:"result"`
	Findings []ExpectedFinding `json:"findings"`
}

type Fixture struct {
	Name               string
	Base, A, B         map[string]string
	ADeleted, BDeleted []string
	Expected           Expected
}

type section struct {
	name string
	body string
}

func splitSections(s string) []section {
	var out []section
	for _, line := range strings.SplitAfter(s, "\n") {
		trimmed := strings.TrimRight(line, "\n")
		if len(trimmed) > 6 && strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			out = append(out, section{name: strings.TrimSpace(trimmed[3 : len(trimmed)-3])})
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].body += line
		}
	}
	return out
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func Parse(name string, data []byte) (*Fixture, error) {
	f := &Fixture{Name: name, Base: map[string]string{}, A: map[string]string{}, B: map[string]string{}}
	haveExpected := false
	for _, s := range splitSections(string(data)) {
		switch {
		case s.name == "expected.json":
			if err := json.Unmarshal([]byte(s.body), &f.Expected); err != nil {
				return nil, fmt.Errorf("%s: expected.json: %w", name, err)
			}
			haveExpected = true
		case s.name == "a.deleted":
			f.ADeleted = lines(s.body)
		case s.name == "b.deleted":
			f.BDeleted = lines(s.body)
		case strings.HasPrefix(s.name, "base/"):
			f.Base[strings.TrimPrefix(s.name, "base/")] = s.body
		case strings.HasPrefix(s.name, "a/"):
			f.A[strings.TrimPrefix(s.name, "a/")] = s.body
		case strings.HasPrefix(s.name, "b/"):
			f.B[strings.TrimPrefix(s.name, "b/")] = s.body
		default:
			return nil, fmt.Errorf("%s: unknown section %q", name, s.name)
		}
	}
	if !haveExpected {
		return nil, fmt.Errorf("%s: missing expected.json section", name)
	}
	return f, nil
}

// LoadDir parses every *.txtar file in dir, sorted by name.
func LoadDir(dir string) ([]*Fixture, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.txtar"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []*Fixture
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f, err := Parse(strings.TrimSuffix(filepath.Base(p), ".txtar"), data)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Build creates the repo: base on main, then branches a and b.
func (f *Fixture) Build(t testing.TB) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	for _, p := range sortedKeys(f.Base) {
		r.Write(p, f.Base[p])
	}
	r.Commit("base")
	sides := []struct {
		name  string
		files map[string]string
		del   []string
	}{{"a", f.A, f.ADeleted}, {"b", f.B, f.BDeleted}}
	for _, s := range sides {
		r.Checkout("main")
		r.Branch(s.name)
		for _, p := range sortedKeys(s.files) {
			r.Write(p, s.files[p])
		}
		for _, p := range s.del {
			r.Remove(p)
		}
		r.Commit(s.name)
	}
	r.Checkout("main")
	return r
}

func touched(files map[string]string, deleted []string) map[string]bool {
	m := map[string]bool{}
	for p := range files {
		m[p] = true
	}
	for _, p := range deleted {
		m[p] = true
	}
	return m
}

// ZeroOverlap reports whether a and b touch disjoint sets of files.
func (f *Fixture) ZeroOverlap() bool {
	a := touched(f.A, f.ADeleted)
	for p := range touched(f.B, f.BDeleted) {
		if a[p] {
			return false
		}
	}
	return true
}

// Positive reports whether the fixture expects any finding.
func (f *Fixture) Positive() bool { return len(f.Expected.Findings) > 0 }

func (e ExpectedFinding) matches(f verdict.Finding) bool {
	if e.Class != f.Class {
		return false
	}
	if e.Kind != "" && e.Kind != f.Kind {
		return false
	}
	if e.Symbol != "" && e.Symbol != f.Symbol {
		return false
	}
	if e.RenamedTo != "" && e.RenamedTo != f.RenamedTo {
		return false
	}
	if e.MinConfidence != "" && f.Confidence.Rank() < e.MinConfidence.Rank() {
		return false
	}
	if e.MaxConfidence != "" && f.Confidence.Rank() > e.MaxConfidence.Rank() {
		return false
	}
	return true
}

// Score returns one message per mismatch between the verdict and expectations.
func Score(f *Fixture, v *verdict.Verdict) []string {
	var fails []string
	if v.Result != f.Expected.Result {
		fails = append(fails, fmt.Sprintf("result = %s, want %s", v.Result, f.Expected.Result))
	}
	used := make([]bool, len(v.Findings))
	for _, e := range f.Expected.Findings {
		found := false
		for i, got := range v.Findings {
			if !used[i] && e.matches(got) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			fails = append(fails, fmt.Sprintf("missing finding %+v", e))
		}
	}
	for i, got := range v.Findings {
		if !used[i] {
			fails = append(fails, fmt.Sprintf("unexpected finding %s/%s %q at confidence %s", got.Class, got.Kind, got.Symbol, got.Confidence))
		}
	}
	return fails
}
