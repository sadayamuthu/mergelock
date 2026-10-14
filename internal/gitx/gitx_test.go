package gitx

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/gittest"
)

func TestParseVersion(t *testing.T) {
	maj, min, err := parseVersion("git version 2.50.1 (Apple Git-155)\n")
	if err != nil || maj != 2 || min != 50 {
		t.Fatalf("parseVersion = %d.%d, %v", maj, min, err)
	}
	if _, _, err := parseVersion("nonsense"); err == nil {
		t.Fatal("expected error for unparseable version")
	}
	if err := RequireVersion(2, 38); err != nil {
		t.Fatalf("installed git must be >= 2.38 to run these tests: %v", err)
	}
	if err := RequireVersion(99, 0); err == nil || !strings.Contains(err.Error(), "99.0 or later") {
		t.Fatalf("RequireVersion(99,0) = %v", err)
	}
}

func TestOpenNotRepo(t *testing.T) {
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("Open(non-repo) = %v, want ErrNotRepo", err)
	}
}

func TestOpenFromSubdirAndWorktree(t *testing.T) {
	r := gittest.New(t)
	r.Write("src/a.ts", "export const a = 1;\n")
	r.Commit("base")
	sub, err := Open(filepath.Join(r.Dir, "src"))
	if err != nil {
		t.Fatal(err)
	}
	if sub.Dir != r.Dir {
		t.Errorf("Dir from subdir = %q, want %q", sub.Dir, r.Dir)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	r.Git("worktree", "add", "-q", "-b", "agent-1/side", wt)
	w, err := Open(wt)
	if err != nil {
		t.Fatal(err)
	}
	wantCommon := filepath.Join(r.Dir, ".git")
	gotCommon, _ := filepath.EvalSymlinks(w.CommonDir)
	if gotCommon != wantCommon {
		t.Errorf("worktree CommonDir = %q, want %q", gotCommon, wantCommon)
	}
}

func TestResolveCommitSlashBranch(t *testing.T) {
	r := gittest.New(t)
	r.Commit("base")
	r.Branch("agent-1/users-api")
	sha := r.Commit("work")
	repo, err := Open(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.ResolveCommit("agent-1/users-api")
	if err != nil || got != sha {
		t.Fatalf("ResolveCommit(slash branch) = %q, %v; want %q", got, err, sha)
	}
	got, err = repo.ResolveCommit(sha[:7])
	if err != nil || got != sha {
		t.Fatalf("ResolveCommit(short sha) = %q, %v", got, err)
	}
	if _, err := repo.ResolveCommit("does-not-exist"); err == nil || !strings.Contains(err.Error(), `unknown ref "does-not-exist"`) {
		t.Fatalf("ResolveCommit(missing) = %v", err)
	}
}

func TestMergeBases(t *testing.T) {
	r := gittest.New(t)
	base := r.Commit("base")
	r.Branch("a")
	r.Commit("a")
	r.Checkout("main")
	r.Branch("b")
	r.Commit("b")
	r.Git("checkout", "-q", "--orphan", "orphan")
	r.Commit("orphan root")
	repo, _ := Open(r.Dir)
	got, err := repo.MergeBases("a", "b")
	if err != nil || len(got) != 1 || got[0] != base {
		t.Fatalf("MergeBases(a,b) = %v, %v; want [%s]", got, err, base)
	}
	got, err = repo.MergeBases("a", "orphan")
	if err != nil || len(got) != 0 {
		t.Fatalf("MergeBases(unrelated) = %v, %v; want empty", got, err)
	}
}

func TestMergeTreeConflicts(t *testing.T) {
	r := gittest.New(t)
	r.Write("keep.txt", "a\n")
	r.Write("del.txt", "x\n")
	r.Write("both.txt", "line1\n")
	r.Commit("base")
	r.Branch("A")
	r.Remove("del.txt")
	r.Write("new.txt", "A\n")
	r.Write("both.txt", "lineA\n")
	r.Commit("A")
	r.Checkout("main")
	r.Branch("B")
	r.Write("del.txt", "y\n")
	r.Write("new.txt", "B\n")
	r.Write("both.txt", "lineB\n")
	r.Commit("B")
	r.Checkout("main")
	r.Branch("C")
	r.Write("other.txt", "z\n")
	r.Commit("C")

	repo, _ := Open(r.Dir)
	res, err := repo.MergeTree("A", "B")
	if err != nil {
		t.Fatal(err)
	}
	if res.Clean {
		t.Fatal("A/B should conflict")
	}
	if strings.Join(res.ConflictedPaths, ",") != "both.txt,del.txt,new.txt" {
		t.Errorf("ConflictedPaths = %v", res.ConflictedPaths)
	}
	var msgs []string
	for _, m := range res.Messages {
		msgs = append(msgs, m.Message)
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{
		"CONFLICT (content): Merge conflict in both.txt",
		"CONFLICT (modify/delete): del.txt deleted in A and modified in B.",
		"CONFLICT (add/add): Merge conflict in new.txt",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("messages missing %q:\n%s", want, joined)
		}
	}
	for _, m := range res.Messages {
		if strings.HasSuffix(m.Message, "\n") || len(m.Paths) == 0 {
			t.Errorf("message not normalized: %+v", m)
		}
	}
	clean, err := repo.MergeTree("A", "C")
	if err != nil || !clean.Clean || len(clean.ConflictedPaths) != 0 {
		t.Fatalf("A/C should merge cleanly: %+v, %v", clean, err)
	}
}

func TestDiffTreeRenameAndSpaces(t *testing.T) {
	r := gittest.New(t)
	r.Write("docs/my notes.txt", strings.Repeat("same content line\n", 20))
	r.Write("src/ünï.ts", "export const x = 1;\n")
	r.Write("gone.txt", "bye\n")
	base := r.Commit("base")
	r.Git("mv", "docs/my notes.txt", "docs/our notes.txt")
	r.Write("src/ünï.ts", "export const x = 2;\n")
	r.Remove("gone.txt")
	r.Write("added file.ts", "export {};\n")
	head := r.Commit("change")
	repo, _ := Open(r.Dir)
	changes, err := repo.DiffTree(base, head)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Change{}
	for _, c := range changes {
		got[c.Path] = c
	}
	if c := got["docs/our notes.txt"]; c.Status != 'R' || c.OldPath != "docs/my notes.txt" {
		t.Errorf("rename = %+v", c)
	}
	if c := got["src/ünï.ts"]; c.Status != 'M' {
		t.Errorf("unicode modify = %+v", c)
	}
	if c := got["gone.txt"]; c.Status != 'D' {
		t.Errorf("delete = %+v", c)
	}
	if c := got["added file.ts"]; c.Status != 'A' {
		t.Errorf("add = %+v", c)
	}
	none, err := repo.DiffTree(head, head)
	if err != nil || len(none) != 0 {
		t.Errorf("DiffTree(same) = %v, %v", none, err)
	}
}

func TestListTreeAndBlobReader(t *testing.T) {
	r := gittest.New(t)
	r.Write("dir with space/ünï.ts", "export const x = 1;\n")
	r.Write("b.txt", "")
	head := r.Commit("base")
	repo, _ := Open(r.Dir)
	tree, err := repo.ListTree(head)
	if err != nil {
		t.Fatal(err)
	}
	oid, ok := tree["dir with space/ünï.ts"]
	if !ok || len(oid) != 40 {
		t.Fatalf("ListTree = %v", tree)
	}
	br, err := repo.NewBlobReader()
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	data, ok, err := br.Read(oid)
	if err != nil || !ok || string(data) != "export const x = 1;\n" {
		t.Fatalf("Read = %q, %v, %v", data, ok, err)
	}
	empty, ok, err := br.Read(tree["b.txt"])
	if err != nil || !ok || len(empty) != 0 {
		t.Fatalf("Read(empty) = %q, %v, %v", empty, ok, err)
	}
	_, ok, err = br.Read(strings.Repeat("0", 40))
	if err != nil || ok {
		t.Fatalf("Read(missing) ok=%v err=%v; want ok=false, nil", ok, err)
	}
	// Reader keeps working after a miss.
	if data, ok, _ := br.Read(oid); !ok || len(data) == 0 {
		t.Fatal("reader broken after missing object")
	}
}
