package model

import (
	"testing"
)

func p(name string) Param { return Param{Name: name} }

func TestAccepts(t *testing.T) {
	twoReq := Signature{Params: []Param{p("name"), p("email")}}
	oneOpt := Signature{Params: []Param{p("name"), {Name: "email", Optional: true}}}
	rest := Signature{Params: []Param{p("a"), {Name: "xs", Rest: true}}}
	py := Signature{Params: []Param{p("name"), {Name: "email", Optional: true}, {Name: "key", KwOnly: true}, {Name: "flag", KwOnly: true, Optional: true}}}
	kwargs := Signature{Params: []Param{p("a"), {Name: "kw", KwRest: true}}}
	cases := []struct {
		name   string
		sig    Signature
		args   int
		kws    []string
		spread bool
		want   bool
	}{
		{"exact", twoReq, 2, nil, false, true},
		{"too few", twoReq, 1, nil, false, false},
		{"too many", twoReq, 3, nil, false, false},
		{"optional omitted", oneOpt, 1, nil, false, true},
		{"rest absorbs", rest, 5, nil, false, true},
		{"rest needs required", rest, 0, nil, false, false},
		{"py keyword fills positional", py, 1, []string{"key", "email"}, false, true},
		{"py missing kwonly", py, 2, nil, false, false},
		{"py unknown keyword", py, 1, []string{"key", "nope"}, false, false},
		{"py keyword duplicates positional", py, 2, []string{"name", "key"}, false, false},
		{"kwargs absorbs unknown keyword", kwargs, 1, []string{"anything"}, false, true},
		{"spread unknown length", twoReq, 0, nil, true, true},
		{"spread with too many fixed", twoReq, 3, nil, true, false},
	}
	for _, c := range cases {
		if got := Accepts(c.sig, c.args, c.kws, c.spread); got != c.want {
			t.Errorf("%s: Accepts = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestShinglesAndJaccard(t *testing.T) {
	a := []string{"$id", "(", "$id", ")", "{", "return", "$id", "}"}
	sa := Shingles(a, 3)
	if len(sa) == 0 {
		t.Fatal("no shingles")
	}
	if got := Jaccard(sa, Shingles(a, 3)); got != 1 {
		t.Errorf("identical Jaccard = %v", got)
	}
	b := []string{"if", "(", "$id", ")", "{", "throw", "$lit", "}"}
	if got := Jaccard(sa, Shingles(b, 3)); got >= 0.5 {
		t.Errorf("different Jaccard = %v, want < 0.5", got)
	}
	if Shingles([]string{"a"}, 3) != nil {
		t.Error("too few tokens should give nil")
	}
	if Jaccard(nil, nil) != 0 {
		t.Error("empty Jaccard should be 0")
	}
}

func TestNormalizeType(t *testing.T) {
	cases := map[string]string{
		" Array< string > ":           "Array",
		"Map<string, Set<T>>":         "Map",
		"(a: number) => void":         "(a:number)=>void",
		"Array<() => void>":           "Array",
		"Record<string, (x) => void>": "Record",
		"":                            "",
	}
	for in, want := range cases {
		if got := NormalizeType(in); got != want {
			t.Errorf("NormalizeType(%q) = %q, want %q", in, got, want)
		}
	}
	if HashStrings("a", "bc") == HashStrings("ab", "c") {
		t.Error("HashStrings must separate parts")
	}
}

func TestIdentityAndRebase(t *testing.T) {
	r1 := Reference{FromKey: "src/a.ts::run", Callee: CalleeExpr{Base: "svc", Path: []string{"login"}}, ArgCount: 2, Keywords: []string{"b", "a"}}
	r2 := r1
	r2.FromKey = "src/moved.ts::run"
	r2.Keywords = []string{"a", "b"}
	if r1.Identity() != r2.Identity() {
		t.Error("Identity must ignore file path and keyword order")
	}
	r3 := r1
	r3.ArgCount = 1
	if r1.Identity() == r3.Identity() {
		t.Error("Identity must include arg count")
	}
	if got := (CalleeExpr{Base: "X", New: true}).Text(); got != "new X" {
		t.Errorf("Text = %q", got)
	}
	f := &FileFacts{Path: "old.ts",
		Symbols:    []Symbol{{File: "old.ts", Name: "C.m", Key: "old.ts::C.m"}},
		References: []Reference{{File: "old.ts", FromKey: "old.ts::C.m"}}}
	f.Rebase("new.ts")
	if f.Path != "new.ts" || f.Symbols[0].Key != "new.ts::C.m" || f.References[0].FromKey != "new.ts::C.m" || f.References[0].File != "new.ts" {
		t.Errorf("Rebase = %+v", f)
	}
	if ScopeName("a/b.ts::C.m") != "C.m" {
		t.Error("ScopeName")
	}
}

func TestShape(t *testing.T) {
	cases := map[string]Param{
		"req": {}, "opt": {Optional: true}, "rest": {Rest: true}, "kwrest": {KwRest: true},
		"kw": {KwOnly: true}, "kwopt": {KwOnly: true, Optional: true},
	}
	for want, p := range cases {
		if got := p.Shape(); got != want {
			t.Errorf("Shape(%+v) = %q, want %q", p, got, want)
		}
	}
}
