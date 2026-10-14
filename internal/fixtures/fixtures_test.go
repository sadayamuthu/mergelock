package fixtures

import (
	"context"
	"encoding/json"
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

func TestFixtures(t *testing.T) {
	all, err := LoadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range all {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			v := runFixture(t, f)
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
