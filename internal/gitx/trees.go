package gitx

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// Change is one entry of a tree diff. Status is A, M, D, R, C or T; OldPath
// is set for R and C.
type Change struct {
	Status  byte
	Path    string
	OldPath string
}

// DiffTree lists files changed between two commits, with rename detection.
func (r *Repo) DiffTree(from, to string) ([]Change, error) {
	out, _, err := r.git("diff-tree", "-r", "-M", "--no-commit-id", "--name-status", "-z", from, to)
	if err != nil {
		return nil, err
	}
	return parseDiffTree(out)
}

func parseDiffTree(out []byte) ([]Change, error) {
	s := strings.TrimSuffix(string(out), "\x00")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, "\x00")
	var changes []Change
	for i := 0; i < len(parts); {
		st := parts[i]
		if st == "" {
			return nil, fmt.Errorf("malformed diff-tree output")
		}
		c := Change{Status: st[0]}
		if c.Status == 'R' || c.Status == 'C' {
			if i+2 >= len(parts) {
				return nil, fmt.Errorf("truncated diff-tree output")
			}
			c.OldPath, c.Path = parts[i+1], parts[i+2]
			i += 3
		} else {
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("truncated diff-tree output")
			}
			c.Path = parts[i+1]
			i += 2
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// ListTree maps every blob path in commit to its object id.
func (r *Repo) ListTree(commit string) (map[string]string, error) {
	out, _, err := r.git("ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, e := range strings.Split(string(out), "\x00") {
		tab := strings.IndexByte(e, '\t')
		if tab < 0 {
			continue
		}
		meta := strings.Fields(e[:tab])
		if len(meta) != 3 || meta[1] != "blob" {
			continue
		}
		files[e[tab+1:]] = meta[2]
	}
	return files, nil
}

// BlobReader reads objects through one long-lived `git cat-file --batch`.
// It is not safe for concurrent use.
type BlobReader struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func (r *Repo) NewBlobReader() (*BlobReader, error) {
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = r.Dir
	cmd.Env = env()
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &BlobReader{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 64<<10)}, nil
}

// Read returns the content of a blob. ok is false when the object is missing
// or is not a blob.
func (b *BlobReader) Read(object string) ([]byte, bool, error) {
	if _, err := io.WriteString(b.in, object+"\n"); err != nil {
		return nil, false, err
	}
	header, err := b.out.ReadString('\n')
	if err != nil {
		return nil, false, err
	}
	header = strings.TrimSuffix(header, "\n")
	if strings.HasSuffix(header, " missing") || strings.HasSuffix(header, " ambiguous") {
		return nil, false, nil
	}
	f := strings.Fields(header)
	if len(f) != 3 {
		return nil, false, fmt.Errorf("unexpected cat-file header %q", header)
	}
	size, err := strconv.Atoi(f[2])
	if err != nil {
		return nil, false, fmt.Errorf("unexpected cat-file header %q", header)
	}
	data := make([]byte, size+1) // content plus trailing newline
	if _, err := io.ReadFull(b.out, data); err != nil {
		return nil, false, err
	}
	if f[1] != "blob" {
		return nil, false, nil
	}
	return data[:size], true, nil
}

func (b *BlobReader) Close() error {
	b.in.Close()
	return b.cmd.Wait()
}

// CurrentBranch returns the checked-out branch name.
func (r *Repo) CurrentBranch() (string, error) {
	out, code, err := r.git("symbolic-ref", "--quiet", "--short", "HEAD")
	if code == 1 {
		return "", errors.New("HEAD is detached; pass a branch name")
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RefBlobs returns the content of every ref under prefix that points directly
// at a blob (how claims and verdicts are stored), keyed by full ref name.
func (r *Repo) RefBlobs(prefix string) (map[string][]byte, error) {
	out, _, err := r.git("for-each-ref", "--format=%(objectname) %(objecttype) %(refname)", prefix)
	if err != nil {
		return nil, err
	}
	blobs := map[string][]byte{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.SplitN(line, " ", 3)
		if len(f) != 3 || f[1] != "blob" {
			continue
		}
		data, _, err := r.git("cat-file", "blob", f[0])
		if err != nil {
			return nil, err
		}
		blobs[f[2]] = data
	}
	return blobs, nil
}
