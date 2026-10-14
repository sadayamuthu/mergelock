// Package verdict defines the machine-readable result of a check (schema v1)
// and everything derived from it: ordering, exit codes and text rendering.
package verdict

const SchemaVersion = 1

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

func (s Severity) Rank() int {
	switch s {
	case SeverityError:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	}
	return 0
}

type Confidence string

const (
	ConfidenceLow     Confidence = "low"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceHigh    Confidence = "high"
	ConfidenceCertain Confidence = "certain"
)

func (c Confidence) Rank() int {
	switch c {
	case ConfidenceCertain:
		return 4
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	}
	return 0
}

// Downgrade returns the next lower confidence level. Low stays low.
func (c Confidence) Downgrade() Confidence {
	switch c {
	case ConfidenceCertain:
		return ConfidenceHigh
	case ConfidenceHigh:
		return ConfidenceMedium
	}
	return ConfidenceLow
}

func MaxConfidence(a, b Confidence) Confidence {
	if a.Rank() >= b.Rank() {
		return a
	}
	return b
}

func MinConfidence(a, b Confidence) Confidence {
	if a.Rank() <= b.Rank() {
		return a
	}
	return b
}

type Location struct {
	File string `json:"file"`
	Line int    `json:"line,omitempty"`
	Text string `json:"text,omitempty"`
}

// Finding is one conflict. Fields that do not apply to a class stay zero and
// are omitted from JSON.
type Finding struct {
	Class           string     `json:"class"`
	Kind            string     `json:"kind,omitempty"`
	Severity        Severity   `json:"severity"`
	Confidence      Confidence `json:"confidence"`
	Symbol          string     `json:"symbol,omitempty"`
	Paths           []string   `json:"paths,omitempty"`
	ChangedIn       string     `json:"changed_in,omitempty"`
	ChangedAt       *Location  `json:"changed_at,omitempty"`
	ModifiedAt      *Location  `json:"modified_at,omitempty"`
	OldSignature    string     `json:"old_signature,omitempty"`
	NewSignature    string     `json:"new_signature,omitempty"`
	RenamedTo       string     `json:"renamed_to,omitempty"`
	UsedIn          string     `json:"used_in,omitempty"`
	CallSites       []Location `json:"call_sites,omitempty"`
	Duplicates      []Location `json:"duplicates,omitempty"`
	Similarity      float64    `json:"similarity,omitempty"`
	Messages        []string   `json:"messages,omitempty"`
	SuggestedAction string     `json:"suggested_action"`

	// FromGit marks findings derived from git merge-tree. Their confidence is
	// never downgraded for partial parses.
	FromGit bool `json:"-"`
}

// Files lists every repo path the finding refers to.
func (f Finding) Files() []string {
	var out []string
	out = append(out, f.Paths...)
	if f.ChangedAt != nil {
		out = append(out, f.ChangedAt.File)
	}
	if f.ModifiedAt != nil {
		out = append(out, f.ModifiedAt.File)
	}
	for _, l := range f.CallSites {
		out = append(out, l.File)
	}
	for _, l := range f.Duplicates {
		out = append(out, l.File)
	}
	return out
}

type Coverage struct {
	Languages   []string `json:"languages"`
	Unsupported []string `json:"unsupported"`
	Partial     []string `json:"partial"`
}

type Stats struct {
	SymbolsCompared int   `json:"symbols_compared"`
	RulesRun        int   `json:"rules_run"`
	DurationMS      int64 `json:"duration_ms"`
}

type Result string

const (
	Clean    Result = "clean"
	Warnings Result = "warnings"
	Conflict Result = "conflict"
)

type Verdict struct {
	Version  int       `json:"version"`
	Base     string    `json:"base"`
	Branches []string  `json:"branches"`
	Result   Result    `json:"result"`
	Coverage Coverage  `json:"coverage"`
	Notes    []string  `json:"notes,omitempty"`
	Findings []Finding `json:"findings"`
	Stats    Stats     `json:"stats"`
}
