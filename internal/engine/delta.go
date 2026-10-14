package engine

import (
	"context"
	"sort"

	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/model"
)

type SymbolChange struct {
	Old, New model.Symbol
}

// Delta is what one branch changed relative to the merge base, at symbol level.
type Delta struct {
	Commit          string
	Added           map[string]model.Symbol // side key
	Removed         map[string]model.Symbol // base key -> base symbol
	SigChanged      map[string]SymbolChange // base key
	BodyChanged     map[string]SymbolChange // base key
	Renamed         map[string]string       // removed base key -> added side key
	NewRefs         []model.Reference       // references not present at base
	SymbolsCompared int
}

func computeDelta(ctx context.Context, l *Loader, base, commit string, changes []gitx.Change) (*Delta, error) {
	d := &Delta{
		Commit:      commit,
		Added:       map[string]model.Symbol{},
		Removed:     map[string]model.Symbol{},
		SigChanged:  map[string]SymbolChange{},
		BodyChanged: map[string]SymbolChange{},
		Renamed:     map[string]string{},
	}
	for _, ch := range changes {
		if err := ctx.Err(); err != nil { // stop parsing once the check is cancelled
			return nil, err
		}
		oldPath, newPath := ch.Path, ch.Path
		switch ch.Status {
		case 'A', 'C':
			oldPath = ""
		case 'D':
			newPath = ""
		case 'R':
			oldPath = ch.OldPath
		}
		var oldF, newF *model.FileFacts
		var err error
		if oldPath != "" {
			if oldF, err = l.FactsAt(base, oldPath); err != nil {
				return nil, err
			}
		}
		if newPath != "" {
			if newF, err = l.FactsAt(commit, newPath); err != nil {
				return nil, err
			}
		}
		d.addFile(oldF, newF)
	}
	sort.SliceStable(d.NewRefs, func(i, j int) bool {
		if d.NewRefs[i].File != d.NewRefs[j].File {
			return d.NewRefs[i].File < d.NewRefs[j].File
		}
		return d.NewRefs[i].Line < d.NewRefs[j].Line
	})
	return d, nil
}

func byName(f *model.FileFacts) map[string]model.Symbol {
	m := map[string]model.Symbol{}
	if f != nil {
		for _, s := range f.Symbols {
			m[s.Name] = s
		}
	}
	return m
}

func (d *Delta) addFile(oldF, newF *model.FileFacts) {
	oldSyms, newSyms := byName(oldF), byName(newF)
	var removed, added []model.Symbol
	for name, o := range oldSyms {
		n, ok := newSyms[name]
		if !ok {
			d.Removed[o.Key] = o
			removed = append(removed, o)
			continue
		}
		if o.SigHash != n.SigHash {
			d.SigChanged[o.Key] = SymbolChange{Old: o, New: n}
		}
		if o.BodyHash != n.BodyHash {
			d.BodyChanged[o.Key] = SymbolChange{Old: o, New: n}
		}
	}
	for name, n := range newSyms {
		if _, ok := oldSyms[name]; !ok {
			d.Added[n.Key] = n
			added = append(added, n)
		}
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i].Name < removed[j].Name })
	sort.Slice(added, func(i, j int) bool { return added[i].Name < added[j].Name })
	used := map[string]bool{}
	for _, o := range removed {
		if o.BodyHash == "" {
			continue
		}
		for _, n := range added {
			if !used[n.Key] && n.Kind == o.Kind && n.BodyHash == o.BodyHash {
				d.Renamed[o.Key] = n.Key
				used[n.Key] = true
				break
			}
		}
	}
	d.SymbolsCompared += len(oldSyms) + len(newSyms)

	seen := map[string]int{}
	if oldF != nil {
		for _, r := range oldF.References {
			seen[r.Identity()]++
		}
	}
	if newF != nil {
		for _, r := range newF.References {
			id := r.Identity()
			if seen[id] > 0 {
				seen[id]--
				continue
			}
			d.NewRefs = append(d.NewRefs, r)
		}
	}
}
