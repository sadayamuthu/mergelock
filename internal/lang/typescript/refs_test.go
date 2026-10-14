package typescript

import (
	"testing"

	"github.com/sadayamuthu/mergelock/internal/model"
)

const refsSample = `import { AuthService, login as doLogin } from "../auth/service";
import * as util from "./util";
import Def from "./def";
import "./side-effect";
export * from "./barrel";
export { x as y, z } from "./xy";
export class Api {
  private svc: AuthService;
  run(name: string) { return this.check(name); }
  check(n: string) { return n; }
}
export function go(svc: AuthService, other) {
  svc.login("a", "b");
  other.login("a");
  doLogin(...args);
  util.helpers.fmt(1, 2, 3);
  const s = new AuthService(db);
  s.refresh();
  const run = () => s.login("x");
  return Def();
}
export const arrow = (a: number) => util.go(a /* note */);
doLogin("top");
`

func ref(t *testing.T, f *model.FileFacts, text string) model.Reference {
	t.Helper()
	for _, r := range f.References {
		if r.Callee.Text() == text {
			return r
		}
	}
	t.Fatalf("reference %q not found in %+v", text, f.References)
	return model.Reference{}
}

func TestImportsAndReExports(t *testing.T) {
	f := parse(t, "src/api.ts", refsSample)
	want := []model.Import{
		{Local: "AuthService", Imported: "AuthService", Spec: "../auth/service"},
		{Local: "doLogin", Imported: "login", Spec: "../auth/service"},
		{Local: "util", Imported: "*", Spec: "./util"},
		{Local: "Def", Imported: "default", Spec: "./def"},
	}
	if len(f.Imports) != len(want) {
		t.Fatalf("Imports = %+v", f.Imports)
	}
	for i, w := range want {
		if f.Imports[i] != w {
			t.Errorf("Imports[%d] = %+v, want %+v", i, f.Imports[i], w)
		}
	}
	wantRe := []model.ReExport{
		{Name: "*", Imported: "*", Spec: "./barrel"},
		{Name: "y", Imported: "x", Spec: "./xy"},
		{Name: "z", Imported: "z", Spec: "./xy"},
	}
	if len(f.ReExports) != len(wantRe) {
		t.Fatalf("ReExports = %+v", f.ReExports)
	}
	for i, w := range wantRe {
		if f.ReExports[i] != w {
			t.Errorf("ReExports[%d] = %+v, want %+v", i, f.ReExports[i], w)
		}
	}
}

func TestReferences(t *testing.T) {
	f := parse(t, "src/api.ts", refsSample)
	cases := []struct {
		text, from, recv string
		args             int
		spread           bool
	}{
		{"this.check", "Api.run", "", 1, false},
		{"svc.login", "go", "AuthService", 2, false},
		{"other.login", "go", "", 1, false},
		{"doLogin", "go", "", 0, true},
		{"util.helpers.fmt", "go", "", 3, false},
		{"new AuthService", "go", "", 1, false},
		{"s.refresh", "go", "AuthService", 0, false},
		{"s.login", "go", "AuthService", 1, false},
		{"Def", "go", "", 0, false},
		{"util.go", "arrow", "", 1, false},
	}
	for _, c := range cases {
		r := ref(t, f, c.text)
		if model.ScopeName(r.FromKey) != c.from || r.Callee.ReceiverType != c.recv || r.ArgCount != c.args || r.Spread != c.spread {
			t.Errorf("%s = from %q recv %q args %d spread %v; want %q %q %d %v",
				c.text, model.ScopeName(r.FromKey), r.Callee.ReceiverType, r.ArgCount, r.Spread, c.from, c.recv, c.args, c.spread)
		}
		if r.File != "src/api.ts" || r.Line == 0 || r.Text == "" {
			t.Errorf("%s location = %+v", c.text, r)
		}
	}
	var top model.Reference
	for _, r := range f.References {
		if r.Callee.Text() == "doLogin" && r.ArgCount == 1 {
			top = r
		}
	}
	if top.FromKey != "src/api.ts::<module>" {
		t.Errorf("top-level call FromKey = %q", top.FromKey)
	}
	if !ref(t, f, "new AuthService").Callee.New {
		t.Error("new_expression must set New")
	}
}
