package verdict

import (
	"fmt"
	"strings"
)

// RenderText renders the human-readable form of a verdict. It is derived
// only from the verdict; nothing here adds information.
func RenderText(v *Verdict) string {
	var b strings.Builder
	a, c := "", ""
	if len(v.Branches) == 2 {
		a, c = v.Branches[0], v.Branches[1]
	}
	fmt.Fprintf(&b, "%s: %s ↔ %s (base %s)\n", v.Result, a, c, short(v.Base))
	if len(v.Findings) == 0 {
		b.WriteString("no findings\n")
	}
	for _, f := range v.Findings {
		subject := f.Symbol
		if subject == "" {
			subject = strings.Join(f.Paths, ", ")
		}
		fmt.Fprintf(&b, "\n%-8s %-8s %s  %s\n", f.Severity, f.Confidence, f.Class, subject)
		if f.Kind != "" {
			fmt.Fprintf(&b, "  kind: %s\n", f.Kind)
		}
		if f.ChangedAt != nil {
			fmt.Fprintf(&b, "  changed in %s at %s:%d\n", f.ChangedIn, f.ChangedAt.File, f.ChangedAt.Line)
		}
		if f.OldSignature != "" {
			fmt.Fprintf(&b, "  %s → %s\n", f.OldSignature, f.NewSignature)
		}
		if f.RenamedTo != "" {
			fmt.Fprintf(&b, "  renamed to %s\n", f.RenamedTo)
		}
		if f.ModifiedAt != nil {
			fmt.Fprintf(&b, "  modified in %s at %s:%d\n", f.UsedIn, f.ModifiedAt.File, f.ModifiedAt.Line)
		}
		if len(f.CallSites) > 0 {
			fmt.Fprintf(&b, "  call sites in %s:\n", f.UsedIn)
			for _, cs := range f.CallSites {
				fmt.Fprintf(&b, "    %s:%d  %s\n", cs.File, cs.Line, cs.Text)
			}
		}
		for _, d := range f.Duplicates {
			fmt.Fprintf(&b, "  defined at %s:%d\n", d.File, d.Line)
		}
		for _, m := range f.Messages {
			fmt.Fprintf(&b, "  git: %s\n", m)
		}
		fmt.Fprintf(&b, "  → %s\n", f.SuggestedAction)
	}
	if len(v.Coverage.Unsupported) > 0 || len(v.Coverage.Partial) > 0 {
		b.WriteString("\ncoverage:")
		if len(v.Coverage.Unsupported) > 0 {
			fmt.Fprintf(&b, " unsupported: %s;", strings.Join(v.Coverage.Unsupported, ", "))
		}
		if len(v.Coverage.Partial) > 0 {
			fmt.Fprintf(&b, " partial: %s;", strings.Join(v.Coverage.Partial, ", "))
		}
		b.WriteString("\n")
	}
	for _, n := range v.Notes {
		fmt.Fprintf(&b, "note: %s\n", n)
	}
	return b.String()
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
