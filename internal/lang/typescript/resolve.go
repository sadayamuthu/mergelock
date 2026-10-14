package typescript

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/sadayamuthu/mergelock/internal/lang"
)

var probeExts = []string{".ts", ".tsx", ".mts", ".cts", ".d.ts", ".js", ".jsx", ".mjs", ".cjs"}

func (a *Adapter) ResolveModule(fromFile, spec string, fs lang.FS) (string, bool) {
	var bases []string
	if spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
		bases = []string{path.Join(path.Dir(fromFile), spec)}
	} else if c := a.loadTSConfig(fs); c != nil {
		bases = c.candidates(spec)
	}
	for _, b := range bases {
		if p, ok := probe(b, fs); ok {
			return p, true
		}
	}
	return "", false
}

func probe(base string, fs lang.FS) (string, bool) {
	if extensions[path.Ext(base)] && fs.Exists(base) {
		return base, true
	}
	switch ext := path.Ext(base); ext {
	case ".js", ".jsx", ".mjs", ".cjs":
		stem := strings.TrimSuffix(base, ext)
		for _, e := range []string{".ts", ".tsx", ".mts", ".cts"} {
			if fs.Exists(stem + e) {
				return stem + e, true
			}
		}
	}
	for _, e := range probeExts {
		if fs.Exists(base + e) {
			return base + e, true
		}
	}
	for _, e := range probeExts {
		if p := base + "/index" + e; fs.Exists(p) {
			return p, true
		}
	}
	return "", false
}

type tsconfig struct {
	baseURL      string
	explicitBase bool
	paths        map[string][]string
}

func (a *Adapter) loadTSConfig(fs lang.FS) *tsconfig {
	data, ok := fs.Read("tsconfig.json")
	if !ok {
		return nil
	}
	key := string(data)
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.tsconfigs[key]; ok {
		return c
	}
	var raw struct {
		CompilerOptions struct {
			BaseURL string              `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	c := &tsconfig{baseURL: "."}
	if err := json.Unmarshal(stripJSONC(data), &raw); err == nil {
		if raw.CompilerOptions.BaseURL != "" {
			c.baseURL, c.explicitBase = path.Clean(raw.CompilerOptions.BaseURL), true
		}
		c.paths = raw.CompilerOptions.Paths
	}
	a.tsconfigs[key] = c
	return c
}

// candidates returns the paths a bare specifier may refer to, most specific
// alias pattern first.
func (c *tsconfig) candidates(spec string) []string {
	patterns := make([]string, 0, len(c.paths))
	for p := range c.paths {
		patterns = append(patterns, p)
	}
	sort.Slice(patterns, func(i, j int) bool {
		if len(patterns[i]) != len(patterns[j]) {
			return len(patterns[i]) > len(patterns[j])
		}
		return patterns[i] < patterns[j]
	})
	var out []string
	for _, pat := range patterns {
		targets := c.paths[pat]
		if star := strings.Index(pat, "*"); star >= 0 {
			pre, suf := pat[:star], pat[star+1:]
			if len(spec) < len(pre)+len(suf) || !strings.HasPrefix(spec, pre) || !strings.HasSuffix(spec, suf) {
				continue
			}
			m := spec[len(pre) : len(spec)-len(suf)]
			for _, t := range targets {
				out = append(out, path.Join(c.baseURL, strings.Replace(t, "*", m, 1)))
			}
		} else if spec == pat {
			for _, t := range targets {
				out = append(out, path.Join(c.baseURL, t))
			}
		}
	}
	if c.explicitBase {
		out = append(out, path.Join(c.baseURL, spec))
	}
	return out
}

var trailingComma = regexp.MustCompile(`,(\s*[}\]])`)

// stripJSONC removes // and /* */ comments outside strings, then trailing commas.
func stripJSONC(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return trailingComma.ReplaceAll(out, []byte("$1"))
}
