package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/config"
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/gittest"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/rules"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

func conflictRepo(t *testing.T, a, b string) *gitx.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write("notes.txt", "status: draft\n")
	r.Commit("base")
	r.Branch(a)
	r.Write("notes.txt", "status: review\n")
	r.Commit("a")
	r.Checkout("main")
	r.Branch(b)
	r.Write("notes.txt", "status: final\n")
	r.Commit("b")
	repo, err := gitx.Open(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestCheckTextualConflict(t *testing.T) {
	repo := conflictRepo(t, "a", "b")
	v, err := engine.Check(context.Background(), repo, "a", "b", engine.Options{Config: config.Default(), Rules: rules.All()})
	if err != nil {
		t.Fatal(err)
	}
	if v.Result != verdict.Conflict || len(v.Findings) != 1 {
		t.Fatalf("verdict = %+v", v)
	}
	f := v.Findings[0]
	if f.Class != "textual" || f.Confidence != verdict.ConfidenceCertain || f.Paths[0] != "notes.txt" || f.SuggestedAction == "" {
		t.Errorf("finding = %+v", f)
	}
	if v.Stats.RulesRun != len(rules.All()) || len(v.Base) != 40 {
		t.Errorf("stats/base = %+v / %q", v.Stats, v.Base)
	}
}

func TestCheckSlashBranchNames(t *testing.T) {
	repo := conflictRepo(t, "agent-1/users-api", "agent-2/auth-refactor")
	v, err := engine.Check(context.Background(), repo, "agent-1/users-api", "agent-2/auth-refactor", engine.Options{Rules: rules.All()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(v.Branches, " ") != "agent-1/users-api agent-2/auth-refactor" {
		t.Errorf("Branches = %v", v.Branches)
	}
}

func TestCheckDisabledRule(t *testing.T) {
	repo := conflictRepo(t, "a", "b")
	cfg, _ := config.Parse([]byte("rules:\n  textual: off\n"))
	v, err := engine.Check(context.Background(), repo, "a", "b", engine.Options{Config: cfg, Rules: rules.All()})
	if err != nil {
		t.Fatal(err)
	}
	if v.Result != verdict.Clean {
		t.Fatalf("textual disabled, want clean: %+v", v.Findings)
	}
}

func TestCheckNoMergeBase(t *testing.T) {
	r := gittest.New(t)
	r.Commit("base")
	r.Git("checkout", "-q", "--orphan", "orphan")
	r.Commit("orphan root")
	repo, _ := gitx.Open(r.Dir)
	_, err := engine.Check(context.Background(), repo, "main", "orphan", engine.Options{Rules: rules.All()})
	if err == nil || !strings.Contains(err.Error(), "no merge base") {
		t.Fatalf("err = %v, want no merge base", err)
	}
	_, err = engine.Check(context.Background(), repo, "main", "nope", engine.Options{Rules: rules.All()})
	if err == nil || !strings.Contains(err.Error(), `unknown ref "nope"`) {
		t.Fatalf("err = %v, want unknown ref", err)
	}
}

func TestCheckCoverage(t *testing.T) {
	r := gittest.New(t)
	r.Write("src/a.ts", "export function f(): number { return 1; }\n")
	r.Write("deploy.rb", "puts 1\n")
	r.Commit("base")
	r.Branch("a")
	r.Write("src/a.ts", "export function f(): number { return 2; }\n\nexport const broken = ;\n")
	r.Commit("a")
	r.Checkout("main")
	r.Branch("b")
	r.Write("deploy.rb", "puts 2\n")
	r.Commit("b")
	repo, _ := gitx.Open(r.Dir)
	v, err := engine.Check(context.Background(), repo, "a", "b", engine.Options{Rules: rules.All()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(v.Coverage.Languages, ",") != "typescript" ||
		strings.Join(v.Coverage.Unsupported, ",") != "deploy.rb" ||
		strings.Join(v.Coverage.Partial, ",") != "src/a.ts" {
		t.Errorf("coverage = %+v", v.Coverage)
	}
	if v.Stats.SymbolsCompared == 0 {
		t.Error("SymbolsCompared = 0")
	}
}
