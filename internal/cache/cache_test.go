package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/cache"
	"github.com/sadayamuthu/mergelock/internal/model"
)

const blob = "0123456789abcdef0123456789abcdef01234567"

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := cache.Open(dir, false)
	f := &model.FileFacts{Path: "a.ts", Lang: "typescript", Symbols: []model.Symbol{{Name: "f", Key: "a.ts::f", Shingles: []uint64{1, 2}}}}
	if _, ok := c.Get("typescript", "1", blob); ok {
		t.Fatal("empty cache hit")
	}
	if err := c.Put("typescript", "1", blob, f); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get("typescript", "1", blob)
	if !ok || got.Symbols[0].Key != "a.ts::f" || got.Symbols[0].Shingles[1] != 2 {
		t.Fatalf("Get = %+v, %v", got, ok)
	}
	if _, ok := c.Get("typescript", "2", blob); ok {
		t.Error("different adapter version must miss")
	}
	want := filepath.Join(dir, "mergelock", "cache", "v1", "typescript@1", "01", blob+".gob")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("cache file not at %s: %v", want, err)
	}
}

func TestGetReturnsIndependentCopies(t *testing.T) {
	c := cache.Open(t.TempDir(), false)
	c.Put("typescript", "1", blob, &model.FileFacts{Path: "a.ts", Symbols: []model.Symbol{{Key: "k"}}})
	a, _ := c.Get("typescript", "1", blob)
	b, _ := c.Get("typescript", "1", blob)
	a.Symbols[0].Key = "mutated"
	if b.Symbols[0].Key != "k" {
		t.Error("two Gets of one blob share Symbols")
	}
}

func TestCorruptEntryIsDropped(t *testing.T) {
	dir := t.TempDir()
	c := cache.Open(dir, false)
	c.Put("typescript", "1", blob, &model.FileFacts{Path: "a.ts"})
	p := filepath.Join(dir, "mergelock", "cache", "v1", "typescript@1", "01", blob+".gob")
	os.WriteFile(p, []byte("garbage"), 0o644)
	if _, ok := c.Get("typescript", "1", blob); ok {
		t.Fatal("corrupt entry must miss")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("corrupt entry must be deleted")
	}
}

func TestDisabled(t *testing.T) {
	dir := t.TempDir()
	c := cache.Open(dir, true)
	if err := c.Put("typescript", "1", blob, &model.FileFacts{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("typescript", "1", blob); ok {
		t.Error("disabled cache must miss")
	}
	if _, err := os.Stat(filepath.Join(dir, "mergelock")); !os.IsNotExist(err) {
		t.Error("disabled cache must not write")
	}
}
