package verdict_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/verdict"
)

func TestConfidenceHelpers(t *testing.T) {
	if got := verdict.ConfidenceCertain.Downgrade(); got != verdict.ConfidenceHigh {
		t.Errorf("certain.Downgrade() = %s, want high", got)
	}
	if got := verdict.ConfidenceHigh.Downgrade(); got != verdict.ConfidenceMedium {
		t.Errorf("high.Downgrade() = %s, want medium", got)
	}
	if got := verdict.ConfidenceLow.Downgrade(); got != verdict.ConfidenceLow {
		t.Errorf("low.Downgrade() = %s, want low", got)
	}
	if got := verdict.MaxConfidence(verdict.ConfidenceMedium, verdict.ConfidenceHigh); got != verdict.ConfidenceHigh {
		t.Errorf("Max = %s", got)
	}
	if got := verdict.MinConfidence(verdict.ConfidenceHigh, verdict.ConfidenceMedium); got != verdict.ConfidenceMedium {
		t.Errorf("Min = %s", got)
	}
	if got := verdict.MaxConfidence("", verdict.ConfidenceMedium); got != verdict.ConfidenceMedium {
		t.Errorf("Max with empty = %s", got)
	}
}

func TestFinalizeDedupesSortsAndSetsResult(t *testing.T) {
	v := &verdict.Verdict{Findings: []verdict.Finding{
		{Class: "duplicate_definition", Severity: verdict.SeverityWarning, Confidence: verdict.ConfidenceMedium, Symbol: "slugify",
			Duplicates: []verdict.Location{{File: "b.ts", Line: 1}, {File: "c.ts", Line: 1}}},
		{Class: "signature_caller", Severity: verdict.SeverityError, Confidence: verdict.ConfidenceMedium, Symbol: "login",
			ChangedAt: &verdict.Location{File: "a.ts", Line: 3}},
		{Class: "signature_caller", Severity: verdict.SeverityError, Confidence: verdict.ConfidenceHigh, Symbol: "login",
			ChangedAt: &verdict.Location{File: "a.ts", Line: 3}},
	}}
	verdict.Finalize(v)
	if v.Result != verdict.Conflict {
		t.Fatalf("Result = %s, want conflict", v.Result)
	}
	if len(v.Findings) != 2 {
		t.Fatalf("got %d findings, want 2 after dedupe: %+v", len(v.Findings), v.Findings)
	}
	if v.Findings[0].Class != "signature_caller" || v.Findings[0].Confidence != verdict.ConfidenceHigh {
		t.Errorf("first finding = %+v, want signature_caller kept at high", v.Findings[0])
	}
	if v.Version != verdict.SchemaVersion {
		t.Errorf("Version = %d", v.Version)
	}
	if v.Coverage.Languages == nil || v.Coverage.Unsupported == nil || v.Coverage.Partial == nil {
		t.Errorf("coverage slices must be non-nil: %+v", v.Coverage)
	}
}

func TestResultAndExitCodes(t *testing.T) {
	clean := &verdict.Verdict{}
	verdict.Finalize(clean)
	if clean.Result != verdict.Clean || verdict.ExitCode(clean.Result) != 0 {
		t.Errorf("clean: %s / %d", clean.Result, verdict.ExitCode(clean.Result))
	}
	warn := &verdict.Verdict{Findings: []verdict.Finding{{Class: "duplicate_definition", Severity: verdict.SeverityWarning, Confidence: verdict.ConfidenceMedium}}}
	verdict.Finalize(warn)
	if warn.Result != verdict.Warnings || verdict.ExitCode(warn.Result) != 1 {
		t.Errorf("warnings: %s / %d", warn.Result, verdict.ExitCode(warn.Result))
	}
	if verdict.ExitCode(verdict.Conflict) != 2 || verdict.ExitOperational != 3 {
		t.Errorf("conflict/operational exit codes wrong")
	}
}

func TestJSONShape(t *testing.T) {
	v := &verdict.Verdict{Base: "abc", Branches: []string{"a", "b"}, Findings: []verdict.Finding{{
		Class: "textual", Severity: verdict.SeverityError, Confidence: verdict.ConfidenceCertain,
		Paths: []string{"x.txt"}, SuggestedAction: "fix", FromGit: true,
	}}}
	verdict.Finalize(v)
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, absent := range []string{"old_signature", "call_sites", "from_git", "FromGit", "changed_at", "notes"} {
		if strings.Contains(s, absent) {
			t.Errorf("JSON should omit %q: %s", absent, s)
		}
	}
	for _, present := range []string{`"version":1`, `"result":"conflict"`, `"unsupported":[]`, `"suggested_action":"fix"`} {
		if !strings.Contains(s, present) {
			t.Errorf("JSON missing %q: %s", present, s)
		}
	}
	empty := &verdict.Verdict{Branches: []string{"a", "b"}}
	verdict.Finalize(empty)
	b, _ = json.Marshal(empty)
	if !strings.Contains(string(b), `"findings":[]`) {
		t.Errorf("empty findings must be []: %s", b)
	}
}

func TestRenderText(t *testing.T) {
	v := &verdict.Verdict{
		Base:     "9f3c1ad0000000000000000000000000000000000",
		Branches: []string{"agent-1/users-api", "agent-2/auth-refactor"},
		Findings: []verdict.Finding{{
			Class: "signature_caller", Severity: verdict.SeverityError, Confidence: verdict.ConfidenceHigh,
			Symbol: "AuthService.login", ChangedIn: "agent-2/auth-refactor",
			ChangedAt:       &verdict.Location{File: "src/auth/service.ts", Line: 42},
			OldSignature:    "login(name: string, email: string): Session",
			NewSignature:    "login(opts: LoginOptions): Session",
			UsedIn:          "agent-1/users-api",
			CallSites:       []verdict.Location{{File: "src/api/users.ts", Line: 118, Text: "auth.login(name, email)"}},
			SuggestedAction: "Update 1 call site in agent-1/users-api to the new signature.",
		}},
		Coverage: verdict.Coverage{Unsupported: []string{"scripts/deploy.rb"}},
	}
	verdict.Finalize(v)
	out := verdict.RenderText(v)
	for _, want := range []string{
		"conflict: agent-1/users-api ↔ agent-2/auth-refactor (base 9f3c1ad)",
		"error    high     signature_caller  AuthService.login",
		"changed in agent-2/auth-refactor at src/auth/service.ts:42",
		"login(name: string, email: string): Session → login(opts: LoginOptions): Session",
		"call sites in agent-1/users-api:",
		"src/api/users.ts:118  auth.login(name, email)",
		"→ Update 1 call site",
		"unsupported: scripts/deploy.rb",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
	clean := &verdict.Verdict{Base: "abc", Branches: []string{"a", "b"}}
	verdict.Finalize(clean)
	if out := verdict.RenderText(clean); !strings.Contains(out, "clean: a ↔ b") || !strings.Contains(out, "no findings") {
		t.Errorf("clean text output wrong:\n%s", out)
	}
}
