package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/gittest"
)

func cliRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write("notes.txt", "status: draft\n")
	r.Write("other.txt", "x\n")
	r.Commit("base")
	r.Branch("a")
	r.Write("notes.txt", "status: review\n")
	r.Commit("a")
	r.Checkout("main")
	r.Branch("b")
	r.Write("notes.txt", "status: final\n")
	r.Commit("b")
	r.Checkout("main")
	r.Branch("c")
	r.Write("other.txt", "y\n")
	r.Commit("c")
	r.Checkout("main")
	return r
}

func runCLI(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(context.Background(), args, dir, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCLICheckExitCodesAndJSON(t *testing.T) {
	r := cliRepo(t)
	code, out, _ := runCLI(t, r.Dir, "check", "a", "b")
	if code != 2 {
		t.Fatalf("conflict exit = %d, want 2; out=%s", code, out)
	}
	var v struct {
		Result   string `json:"result"`
		Findings []struct {
			Class string `json:"class"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.Result != "conflict" || v.Findings[0].Class != "textual" {
		t.Fatalf("json = %s (%v)", out, err)
	}
	code, out, _ = runCLI(t, r.Dir, "check", "a", "c")
	if code != 0 || !strings.Contains(out, `"result": "clean"`) {
		t.Fatalf("clean exit = %d, out=%s", code, out)
	}
	// B defaults to the configured target (main): a vs main is clean.
	if code, _, _ := runCLI(t, r.Dir, "check", "a"); code != 0 {
		t.Fatalf("default target exit = %d", code)
	}
}

func TestCLICheckTextFormatFlagAfterArgs(t *testing.T) {
	r := cliRepo(t)
	code, out, _ := runCLI(t, r.Dir, "check", "a", "b", "--format", "text")
	if code != 2 || !strings.HasPrefix(out, "conflict: a ↔ b") {
		t.Fatalf("text output (exit %d):\n%s", code, out)
	}
}

func TestCLIUnknownRefExits3(t *testing.T) {
	r := cliRepo(t)
	code, out, errOut := runCLI(t, r.Dir, "check", "a", "nope")
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(errOut, `unknown ref "nope"`) {
		t.Errorf("stderr = %q", errOut)
	}
	var e map[string]string
	if err := json.Unmarshal([]byte(out), &e); err != nil || !strings.Contains(e["error"], "nope") {
		t.Errorf("stdout should be a JSON error, got %q", out)
	}
	code, out, _ = runCLI(t, r.Dir, "check", "a", "nope", "--format", "text")
	if code != 3 || out != "" {
		t.Errorf("text mode error: exit %d stdout %q", code, out)
	}
}

func TestCLIUsageErrors(t *testing.T) {
	r := cliRepo(t)
	if code, _, _ := runCLI(t, r.Dir); code != 3 {
		t.Errorf("no args exit = %d", code)
	}
	if code, _, errOut := runCLI(t, r.Dir, "frobnicate"); code != 3 || !strings.Contains(errOut, "unknown command") {
		t.Errorf("unknown command: %d %q", code, errOut)
	}
	if code, _, _ := runCLI(t, r.Dir, "check"); code != 3 {
		t.Errorf("check without branch exit = %d", code)
	}
	if code, _, errOut := runCLI(t, r.Dir, "check", "a", "--format", "sarif"); code != 3 || !strings.Contains(errOut, "sarif") {
		t.Errorf("bad format: %d %q", code, errOut)
	}
	if code, _, errOut := runCLI(t, t.TempDir(), "check", "a"); code != 3 || !strings.Contains(errOut, "not a git repository") {
		t.Errorf("non-repo: %d %q", code, errOut)
	}
	code, out, _ := runCLI(t, r.Dir, "version")
	if code != 0 || !strings.HasPrefix(out, "mergelock ") || !strings.Contains(out, "\ngit 2.") || !strings.Contains(out, "adapters: typescript@1 python@1") {
		t.Errorf("version: %d %q", code, out)
	}
}
