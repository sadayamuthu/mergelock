package python

import (
	"path"
	"strings"

	"github.com/sadayamuthu/mergelock/internal/lang"
)

// ResolveModule maps a Python module spec to a file: relative specs resolve
// from the importing file's package; absolute specs from the repo root, then src/.
func (a *Adapter) ResolveModule(fromFile, spec string, fs lang.FS) (string, bool) {
	dots := len(spec) - len(strings.TrimLeft(spec, "."))
	rest := spec[dots:]
	var roots []string
	if dots > 0 {
		dir := path.Dir(fromFile)
		for i := 1; i < dots; i++ {
			dir = path.Dir(dir)
		}
		roots = []string{dir}
	} else {
		roots = []string{".", "src"}
	}
	for _, root := range roots {
		base := root
		if rest != "" {
			base = path.Join(root, strings.ReplaceAll(rest, ".", "/"))
			if fs.Exists(base + ".py") {
				return base + ".py", true
			}
		}
		if p := path.Join(base, "__init__.py"); fs.Exists(p) {
			return p, true
		}
	}
	return "", false
}
