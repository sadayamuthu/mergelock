package engine

import (
	"github.com/sadayamuthu/mergelock/internal/cache"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/model"
)

// maxFileBytes bounds what we parse; larger files are usually generated or
// minified and are reported as unsupported.
const maxFileBytes = 1 << 20

// Loader serves parsed FileFacts for (commit, path), parsing each blob at most
// once per process and once per cache lifetime.
type Loader struct {
	repo  *gitx.Repo
	blobs *gitx.BlobReader
	cache *cache.Cache
	reg   *lang.Registry

	trees map[string]map[string]string // commit -> path -> blob oid
	facts map[string]*model.FileFacts  // commit + NUL + path

	Languages   map[string]bool
	Unsupported map[string]bool
	Partial     map[string]bool
}

func NewLoader(repo *gitx.Repo, blobs *gitx.BlobReader, c *cache.Cache, reg *lang.Registry) *Loader {
	return &Loader{
		repo: repo, blobs: blobs, cache: c, reg: reg,
		trees:       map[string]map[string]string{},
		facts:       map[string]*model.FileFacts{},
		Languages:   map[string]bool{},
		Unsupported: map[string]bool{},
		Partial:     map[string]bool{},
	}
}

func (l *Loader) tree(commit string) (map[string]string, error) {
	if t, ok := l.trees[commit]; ok {
		return t, nil
	}
	t, err := l.repo.ListTree(commit)
	if err != nil {
		return nil, err
	}
	l.trees[commit] = t
	return t, nil
}

// FactsAt returns the facts for path at commit, or nil when the path does
// not exist there, has no adapter, or is too large to parse.
func (l *Loader) FactsAt(commit, path string) (*model.FileFacts, error) {
	k := commit + "\x00" + path
	if f, ok := l.facts[k]; ok {
		return f, nil
	}
	f, err := l.load(commit, path)
	if err != nil {
		return nil, err
	}
	l.facts[k] = f
	return f, nil
}

func (l *Loader) load(commit, path string) (*model.FileFacts, error) {
	t, err := l.tree(commit)
	if err != nil {
		return nil, err
	}
	oid, ok := t[path]
	if !ok {
		return nil, nil
	}
	ad := l.reg.For(path)
	if ad == nil {
		l.Unsupported[path] = true
		return nil, nil
	}
	if f, ok := l.cache.Get(ad.Name(), ad.Version(), oid); ok {
		f.Rebase(path)
		l.note(ad, f)
		return f, nil
	}
	data, ok, err := l.blobs.Read(oid)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if len(data) > maxFileBytes {
		l.Unsupported[path] = true
		return nil, nil
	}
	f, parsed := parseSafely(ad, path, data)
	if parsed {
		_ = l.cache.Put(ad.Name(), ad.Version(), oid, f) // cache failures are never fatal
	}
	l.note(ad, f)
	return f, nil
}

// parseSafely never fails: a parser error or panic yields empty, partial
// facts and parsed=false. Such fallbacks are never cached, so a fixed adapter
// (or a transient failure) does not leave a poisoned entry behind.
func parseSafely(ad lang.Adapter, path string, data []byte) (f *model.FileFacts, parsed bool) {
	defer func() {
		if r := recover(); r != nil {
			f, parsed = &model.FileFacts{Path: path, Lang: ad.Name(), Partial: true}, false
		}
	}()
	f, err := ad.Parse(path, data)
	if err != nil || f == nil {
		return &model.FileFacts{Path: path, Lang: ad.Name(), Partial: true}, false
	}
	return f, true
}

func (l *Loader) note(ad lang.Adapter, f *model.FileFacts) {
	l.Languages[ad.Name()] = true
	if f.Partial {
		l.Partial[f.Path] = true
	}
}

// FS exposes one revision to adapters for module resolution.
func (l *Loader) FS(commit string) lang.FS { return revFS{l: l, commit: commit} }

type revFS struct {
	l      *Loader
	commit string
}

func (r revFS) Exists(path string) bool {
	t, err := r.l.tree(r.commit)
	return err == nil && t[path] != ""
}

func (r revFS) Read(path string) ([]byte, bool) {
	t, err := r.l.tree(r.commit)
	if err != nil || t[path] == "" {
		return nil, false
	}
	data, ok, err := r.l.blobs.Read(t[path])
	return data, ok && err == nil
}
