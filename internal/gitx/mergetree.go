package gitx

import (
	"fmt"
	"strconv"
	"strings"
)

// ConflictMessage is one informational or conflict message from merge-tree.
type ConflictMessage struct {
	Paths   []string
	Type    string // merge-tree's machine type, e.g. "CONFLICT (contents)"
	Message string // human message, e.g. "CONFLICT (add/add): Merge conflict in x"
}

type MergeTreeResult struct {
	Clean           bool
	ConflictedPaths []string
	Messages        []ConflictMessage
}

// MergeTree runs a dry merge of a and b without touching the working tree.
func (r *Repo) MergeTree(a, b string) (*MergeTreeResult, error) {
	out, code, err := r.git("merge-tree", "--write-tree", "--name-only", "--messages", "-z", "--end-of-options", a, b)
	// Exit 1 means "conflicts" only when a tree was written; merge-tree also
	// exits 1 with no output for an unusable revision.
	if (code != 0 && code != 1) || (code == 1 && len(out) == 0) {
		return nil, err
	}
	res, perr := parseMergeTree(out)
	if perr != nil {
		return nil, perr
	}
	res.Clean = code == 0
	return res, nil
}

// parseMergeTree parses `git merge-tree --write-tree --name-only --messages -z`:
// <tree>NUL <path>NUL ... NUL then repeated <n>NUL <path>NUL*n <type>NUL <message>NUL.
func parseMergeTree(out []byte) (*MergeTreeResult, error) {
	parts := strings.Split(string(out), "\x00")
	res := &MergeTreeResult{}
	i := 1
	for i < len(parts) && parts[i] != "" {
		res.ConflictedPaths = append(res.ConflictedPaths, parts[i])
		i++
	}
	i++
	for i < len(parts) && parts[i] != "" {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("malformed merge-tree output at %q", parts[i])
		}
		i++
		if i+n+1 >= len(parts) {
			return nil, fmt.Errorf("truncated merge-tree output")
		}
		m := ConflictMessage{Paths: append([]string(nil), parts[i:i+n]...)}
		i += n
		m.Type = parts[i]
		m.Message = strings.TrimRight(parts[i+1], "\n")
		i += 2
		res.Messages = append(res.Messages, m)
	}
	return res, nil
}
