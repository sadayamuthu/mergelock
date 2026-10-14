package rules

import "testing"

func TestClassifyConflict(t *testing.T) {
	cases := map[string]string{
		"CONFLICT (content): Merge conflict in both.txt":                               "textual",
		"CONFLICT (add/add): Merge conflict in new.txt":                                "add_add",
		"CONFLICT (modify/delete): del.txt deleted in A and modified in B.  Version B": "modify_delete",
		"CONFLICT (rename/delete): a.txt renamed to b.txt in A, but deleted in B.":     "modify_delete",
		"CONFLICT (file/directory): directory in the way of x from A":                  "textual",
		"Auto-merging both.txt": "",
	}
	for msg, want := range cases {
		if got := classifyConflict(msg); got != want {
			t.Errorf("classifyConflict(%q) = %q, want %q", msg, got, want)
		}
	}
}
