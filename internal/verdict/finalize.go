package verdict

import (
	"fmt"
	"sort"
	"strings"
)

const ExitOperational = 3

// ExitCode maps a result to the CLI exit code.
func ExitCode(r Result) int {
	switch r {
	case Warnings:
		return 1
	case Conflict:
		return 2
	}
	return 0
}

// Finalize dedupes and sorts findings, computes the result and makes every
// list field non-nil so JSON never contains null arrays.
func Finalize(v *Verdict) {
	v.Version = SchemaVersion
	v.Findings = dedupe(v.Findings)
	sort.SliceStable(v.Findings, func(i, j int) bool { return less(v.Findings[i], v.Findings[j]) })
	v.Result = Clean
	for _, f := range v.Findings {
		if f.Severity == SeverityError {
			v.Result = Conflict
			break
		}
		v.Result = Warnings
	}
	if v.Findings == nil {
		v.Findings = []Finding{}
	}
	if v.Branches == nil {
		v.Branches = []string{}
	}
	if v.Coverage.Languages == nil {
		v.Coverage.Languages = []string{}
	}
	if v.Coverage.Unsupported == nil {
		v.Coverage.Unsupported = []string{}
	}
	if v.Coverage.Partial == nil {
		v.Coverage.Partial = []string{}
	}
}

func anchor(f Finding) (string, int) {
	switch {
	case f.ChangedAt != nil:
		return f.ChangedAt.File, f.ChangedAt.Line
	case len(f.Paths) > 0:
		return f.Paths[0], 0
	case len(f.Duplicates) > 0:
		return f.Duplicates[0].File, f.Duplicates[0].Line
	case len(f.CallSites) > 0:
		return f.CallSites[0].File, f.CallSites[0].Line
	}
	return "", 0
}

func identity(f Finding) string {
	file, line := anchor(f)
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s:%d|%s", f.Class, f.Kind, f.Symbol, f.ChangedIn, f.UsedIn, file, line, strings.Join(f.Files(), ","))
}

// dedupe keeps one finding per identity, preferring the highest confidence.
func dedupe(in []Finding) []Finding {
	index := map[string]int{}
	var out []Finding
	for _, f := range in {
		id := identity(f)
		if i, ok := index[id]; ok {
			if f.Confidence.Rank() > out[i].Confidence.Rank() {
				out[i] = f
			}
			continue
		}
		index[id] = len(out)
		out = append(out, f)
	}
	return out
}

func less(a, b Finding) bool {
	if a.Severity.Rank() != b.Severity.Rank() {
		return a.Severity.Rank() > b.Severity.Rank()
	}
	if a.Confidence.Rank() != b.Confidence.Rank() {
		return a.Confidence.Rank() > b.Confidence.Rank()
	}
	af, al := anchor(a)
	bf, bl := anchor(b)
	if af != bf {
		return af < bf
	}
	if al != bl {
		return al < bl
	}
	if a.Class != b.Class {
		return a.Class < b.Class
	}
	return a.Symbol < b.Symbol
}
