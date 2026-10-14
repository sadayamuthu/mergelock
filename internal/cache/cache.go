// Package cache stores parsed FileFacts on disk keyed by git blob id and
// adapter version. Failures are never fatal: a bad entry is a cache miss.
package cache

import (
	"bufio"
	"encoding/gob"
	"os"
	"path/filepath"

	"github.com/sadayamuthu/mergelock/internal/model"
)

type Cache struct {
	dir      string
	disabled bool
}

// Open returns a cache under <commonDir>/mergelock/cache/v1. All worktrees of
// a repository share commonDir and therefore the cache.
func Open(commonDir string, disabled bool) *Cache {
	return &Cache{dir: filepath.Join(commonDir, "mergelock", "cache", "v1"), disabled: disabled}
}

func (c *Cache) path(adapter, version, blob string) string {
	return filepath.Join(c.dir, adapter+"@"+version, blob[:2], blob+".gob")
}

// Get decodes a fresh FileFacts on every call, so callers may mutate the
// result (for example via Rebase) without affecting other reads.
func (c *Cache) Get(adapter, version, blob string) (*model.FileFacts, bool) {
	if c.disabled || len(blob) < 3 {
		return nil, false
	}
	p := c.path(adapter, version, blob)
	fh, err := os.Open(p)
	if err != nil {
		return nil, false
	}
	defer fh.Close()
	var f model.FileFacts
	if err := gob.NewDecoder(bufio.NewReader(fh)).Decode(&f); err != nil {
		os.Remove(p)
		return nil, false
	}
	return &f, true
}

func (c *Cache) Put(adapter, version, blob string, f *model.FileFacts) error {
	if c.disabled || len(blob) < 3 {
		return nil
	}
	p := c.path(adapter, version, blob)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(tmp).Encode(f); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), p)
}
