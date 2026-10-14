package rules

import (
	"fmt"
	"strings"

	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// classifyConflict maps a merge-tree message to a verdict class (D4). Note
// that merge-tree's machine type for add/add is "CONFLICT (contents)", so we
// classify on the human message.
func classifyConflict(msg string) string {
	switch {
	case strings.HasPrefix(msg, "CONFLICT (add/add)"):
		return "add_add"
	case strings.HasPrefix(msg, "CONFLICT (modify/delete)"), strings.HasPrefix(msg, "CONFLICT (rename/delete)"):
		return "modify_delete"
	case strings.HasPrefix(msg, "CONFLICT"):
		return "textual"
	}
	return ""
}

// mergeTreeFindings turns merge-tree conflict messages of one class into findings.
func mergeTreeFindings(rc *engine.RuleContext, class string) []verdict.Finding {
	if !rc.Primary || rc.MergeTree == nil {
		return nil
	}
	var out []verdict.Finding
	for _, m := range rc.MergeTree.Messages {
		if classifyConflict(m.Message) != class {
			continue
		}
		paths := strings.Join(m.Paths, ", ")
		f := verdict.Finding{
			Class:      class,
			Severity:   verdict.SeverityError,
			Confidence: verdict.ConfidenceCertain,
			Paths:      m.Paths,
			Messages:   []string{m.Message},
			FromGit:    true,
		}
		switch class {
		case "textual":
			f.SuggestedAction = fmt.Sprintf("Both branches edit the same lines in %s; merge one branch first, then rebase the other onto it.", paths)
		case "add_add":
			f.SuggestedAction = fmt.Sprintf("Both branches create %s with different content; keep one version and rebase the other branch onto it.", paths)
		case "modify_delete":
			f.Kind = "file"
			f.SuggestedAction = fmt.Sprintf("One branch deletes %s while the other modifies it; decide whether it should survive, then rebase.", paths)
		}
		out = append(out, f)
	}
	return out
}
