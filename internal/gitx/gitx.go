// Package gitx runs the git binary. It is the only package that talks to git.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ErrNotRepo = errors.New("not a git repository")

// Error is a git invocation that exited non-zero.
type Error struct {
	Args   []string
	Code   int
	Stderr string
}

func (e *Error) Error() string {
	return fmt.Sprintf("git %s: exit %d: %s", strings.Join(e.Args, " "), e.Code, strings.TrimSpace(e.Stderr))
}

// Repo is a git working tree. Dir is the top-level directory; CommonDir is
// the git dir shared by all worktrees (where the parse cache lives).
type Repo struct {
	Dir       string
	CommonDir string
}

func env() []string {
	return append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
}

func run(dir string, stdin io.Reader, args ...string) ([]byte, int, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env()
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return stdout.Bytes(), ee.ExitCode(), &Error{Args: args, Code: ee.ExitCode(), Stderr: stderr.String()}
	}
	if err != nil {
		return nil, -1, err
	}
	return stdout.Bytes(), 0, nil
}

func (r *Repo) git(args ...string) ([]byte, int, error) {
	return run(r.Dir, nil, args...)
}

// Open finds the repository containing dir.
func Open(dir string) (*Repo, error) {
	out, _, err := run(dir, nil, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	if err != nil {
		var ge *Error
		if errors.As(err, &ge) && ge.Code == 128 {
			return nil, ErrNotRepo
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return nil, fmt.Errorf("unexpected rev-parse output %q", out)
	}
	return &Repo{Dir: lines[0], CommonDir: lines[1]}, nil
}

var versionRE = regexp.MustCompile(`git version (\d+)\.(\d+)`)

func parseVersion(s string) (int, int, error) {
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("cannot parse git version from %q", strings.TrimSpace(s))
	}
	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	return maj, min, nil
}

// Version returns the installed git's major and minor version.
func Version() (int, int, error) {
	out, _, err := run("", nil, "version")
	if err != nil {
		return 0, 0, err
	}
	return parseVersion(string(out))
}

// RequireVersion fails unless git is at least major.minor.
func RequireVersion(major, minor int) error {
	maj, min, err := Version()
	if err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	if maj < major || (maj == major && min < minor) {
		return fmt.Errorf("git %d.%d or later is required (found %d.%d)", major, minor, maj, min)
	}
	return nil
}

// ResolveCommit turns a branch name, tag or SHA into a full commit SHA.
func (r *Repo) ResolveCommit(ref string) (string, error) {
	out, code, err := r.git("rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	if err != nil {
		if code > 0 {
			return "", fmt.Errorf("unknown ref %q", ref)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// MergeBases returns all merge bases of a and b; empty when histories are unrelated.
func (r *Repo) MergeBases(a, b string) ([]string, error) {
	out, code, err := r.git("merge-base", "--all", "--end-of-options", a, b)
	if code == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}
