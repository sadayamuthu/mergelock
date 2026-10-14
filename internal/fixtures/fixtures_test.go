package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/config"
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/rules"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

const fixtureDir = "../../testdata/fixtures"

func TestParse(t *testing.T) {
	src := "comment line\n-- base/x.ts --\nexport const x = 1;\n-- a/x.ts --\nexport const x = 2;\n-- b.deleted --\nx.ts\n\n-- expected.json --\n{\"result\":\"clean\",\"findings\":[]}\n"
	f, err := Parse("demo", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if f.Base["x.ts"] != "export const x = 1;\n" || f.A["x.ts"] != "export const x = 2;\n" {
		t.Errorf("files = %+v / %+v", f.Base, f.A)
	}
	if len(f.BDeleted) != 1 || f.BDeleted[0] != "x.ts" {
		t.Errorf("BDeleted = %v", f.BDeleted)
	}
	if f.ZeroOverlap() {
		t.Error("a and b both touch x.ts; not zero overlap")
	}
	if _, err := Parse("bad", []byte("-- base/x --\n1\n")); err == nil {
		t.Error("missing expected.json must be an error")
	}
	if _, err := Parse("bad", []byte("-- nonsense --\n\n-- expected.json --\n{}\n")); err == nil {
		t.Error("unknown section must be an error")
	}
}

func TestScore(t *testing.T) {
	f := &Fixture{Expected: Expected{Result: verdict.Conflict, Findings: []ExpectedFinding{
		{Class: "signature_caller", Symbol: "login", MinConfidence: verdict.ConfidenceHigh},
	}}}
	good := &verdict.Verdict{Result: verdict.Conflict, Findings: []verdict.Finding{
		{Class: "signature_caller", Symbol: "login", Confidence: verdict.ConfidenceHigh},
	}}
	if fails := Score(f, good); len(fails) != 0 {
		t.Errorf("good verdict failed: %v", fails)
	}
	weak := &verdict.Verdict{Result: verdict.Conflict, Findings: []verdict.Finding{
		{Class: "signature_caller", Symbol: "login", Confidence: verdict.ConfidenceMedium},
	}}
	if fails := Score(f, weak); len(fails) == 0 {
		t.Error("confidence below min_confidence must fail")
	}
	extra := &verdict.Verdict{Result: verdict.Conflict, Findings: append(good.Findings, verdict.Finding{Class: "textual"})}
	if fails := Score(f, extra); len(fails) == 0 {
		t.Error("unexpected extra finding must fail")
	}
}

// runFixture builds the fixture repo and checks a against b.
func runFixture(t *testing.T, f *Fixture) *verdict.Verdict {
	t.Helper()
	r := f.Build(t)
	repo, err := gitx.Open(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	v, err := engine.Check(context.Background(), repo, "a", "b", engine.Options{Config: cfg, Rules: rules.All()})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestTally(t *testing.T) {
	f := &Fixture{Expected: Expected{Result: verdict.Conflict, Findings: []ExpectedFinding{
		{Class: "signature_caller"}, {Class: "modify_delete"},
	}}}
	v := &verdict.Verdict{Findings: []verdict.Finding{{Class: "signature_caller"}, {Class: "textual"}}}
	got := Tally(f, v)
	if got["signature_caller"] != (ClassScore{TP: 1}) || got["textual"] != (ClassScore{FP: 1}) || got["modify_delete"] != (ClassScore{FN: 1}) {
		t.Errorf("Tally = %+v", got)
	}
}

func TestFixtures(t *testing.T) {
	all, err := LoadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	totals := map[string]ClassScore{}
	t.Cleanup(func() {
		classes := make([]string, 0, len(totals))
		for c := range totals {
			classes = append(classes, c)
		}
		sort.Strings(classes)
		var b strings.Builder
		b.WriteString("\nclass                  TP  FP  FN  precision  recall\n")
		for _, c := range classes {
			s := totals[c]
			p, r := 1.0, 1.0
			if s.TP+s.FP > 0 {
				p = float64(s.TP) / float64(s.TP+s.FP)
			}
			if s.TP+s.FN > 0 {
				r = float64(s.TP) / float64(s.TP+s.FN)
			}
			fmt.Fprintf(&b, "%-22s %3d %3d %3d  %9.2f  %6.2f\n", c, s.TP, s.FP, s.FN, p, r)
		}
		t.Log(b.String())
	})
	for _, f := range all {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			v := runFixture(t, f)
			mu.Lock()
			for c, s := range Tally(f, v) {
				cur := totals[c]
				totals[c] = ClassScore{TP: cur.TP + s.TP, FP: cur.FP + s.FP, FN: cur.FN + s.FN}
			}
			mu.Unlock()
			if fails := Score(f, v); len(fails) > 0 {
				out, _ := json.MarshalIndent(v, "", "  ")
				for _, msg := range fails {
					t.Error(msg)
				}
				t.Logf("verdict:\n%s", out)
			}
		})
	}
}

func TestSuiteComposition(t *testing.T) {
	all, err := LoadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	zeroOverlap, negatives, ts, py := 0, 0, 0, 0
	for _, f := range all {
		switch {
		case !f.Positive():
			negatives++
		case f.ZeroOverlap():
			zeroOverlap++
		}
		switch {
		case strings.HasPrefix(f.Name, "ts-"):
			ts++
		case strings.HasPrefix(f.Name, "py-"):
			py++
		}
	}
	t.Logf("fixtures=%d zero-overlap positives=%d negatives=%d typescript=%d python=%d", len(all), zeroOverlap, negatives, ts, py)
	if len(all) < 30 {
		t.Errorf("need at least 30 fixtures, have %d", len(all))
	}
	if zeroOverlap < 10 {
		t.Errorf("need at least 10 zero-file-overlap positives, have %d", zeroOverlap)
	}
	if negatives < 8 {
		t.Errorf("need at least 8 clean negatives, have %d", negatives)
	}
}

func TestVerdictsAreDeterministic(t *testing.T) {
	all, err := LoadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range all {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			r := f.Build(t)
			repo, err := gitx.Open(r.Dir)
			if err != nil {
				t.Fatal(err)
			}
			render := func(noCache bool) string {
				v, err := engine.Check(context.Background(), repo, "a", "b", engine.Options{Rules: rules.All(), NoCache: noCache})
				if err != nil {
					t.Fatal(err)
				}
				v.Stats.DurationMS = 0
				b, _ := json.Marshal(v)
				return string(b)
			}
			// cold cache (fills it), no cache, then warm cache (served from it)
			first, second, third := render(false), render(true), render(false)
			if first != second {
				t.Errorf("cold and no-cache verdicts differ:\n%s\n%s", first, second)
			}
			if first != third {
				t.Errorf("cold and warm-cache verdicts differ:\n%s\n%s", first, third)
			}
		})
	}
}
