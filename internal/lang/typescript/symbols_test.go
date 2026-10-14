package typescript

import (
	"testing"

	"github.com/sadayamuthu/mergelock/internal/model"
)

const sample = `import { Db } from "./db";
export class AuthService {
  private db: Db;
  constructor(db: Db, opt?: number) {}
  login(name: string, email = "", ...rest: string[]): Session { return this.check(name); }
  static make(): AuthService { return new AuthService(db); }
}
export interface LoginOptions { name: string }
export type Id = string;
export function helper<T>(a: T, b?: number): T { return a; }
export const arrow = (a: number) => a + 1;
const single = x => x;
function internal() {}
export default function main() {}
export { internal };
`

func parse(t *testing.T, path, src string) *model.FileFacts {
	t.Helper()
	f, err := New().Parse(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func sym(t *testing.T, f *model.FileFacts, name string) model.Symbol {
	t.Helper()
	for _, s := range f.Symbols {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("symbol %q not found in %+v", name, f.Symbols)
	return model.Symbol{}
}

func TestDeclarations(t *testing.T) {
	f := parse(t, "src/auth.ts", sample)
	if f.Lang != "typescript" || f.Partial {
		t.Errorf("Lang/Partial = %s/%v", f.Lang, f.Partial)
	}
	want := map[string]model.Kind{
		"AuthService": model.KindClass, "AuthService.constructor": model.KindMethod,
		"AuthService.login": model.KindMethod, "AuthService.make": model.KindMethod,
		"LoginOptions": model.KindInterface, "Id": model.KindTypeAlias,
		"helper": model.KindFunction, "arrow": model.KindFunction, "single": model.KindFunction,
		"internal": model.KindFunction, "main": model.KindFunction,
	}
	if len(f.Symbols) != len(want) {
		t.Errorf("got %d symbols, want %d: %+v", len(f.Symbols), len(want), f.Symbols)
	}
	for name, kind := range want {
		s := sym(t, f, name)
		if s.Kind != kind || s.Key != "src/auth.ts::"+name || s.File != "src/auth.ts" || s.Lang != "typescript" {
			t.Errorf("%s = %+v", name, s)
		}
	}
	if !sym(t, f, "AuthService.login").Exported || !sym(t, f, "internal").Exported || sym(t, f, "single").Exported {
		t.Error("export flags wrong")
	}
	if f.Default != "main" {
		t.Errorf("Default = %q", f.Default)
	}
	if got := sym(t, f, "AuthService.login").Line; got != 5 {
		t.Errorf("login line = %d", got)
	}
}

func TestSignatures(t *testing.T) {
	f := parse(t, "src/auth.ts", sample)
	login := sym(t, f, "AuthService.login")
	if login.Signature.Text != `login(name: string, email = "", ...rest: string[]): Session` {
		t.Errorf("Text = %q", login.Signature.Text)
	}
	ps := login.Signature.Params
	if len(ps) != 3 || ps[0].Shape() != "req" || ps[1].Shape() != "opt" || ps[2].Shape() != "rest" || ps[2].Name != "rest" {
		t.Errorf("Params = %+v", ps)
	}
	ctor := sym(t, f, "AuthService.constructor").Signature.Params
	if len(ctor) != 2 || !ctor[1].Optional {
		t.Errorf("ctor params = %+v", ctor)
	}
	if p := sym(t, f, "single").Signature.Params; len(p) != 1 || p[0].Name != "x" {
		t.Errorf("single-param arrow = %+v", p)
	}
}

func TestHashes(t *testing.T) {
	base := parse(t, "a.ts", "export function f(name: string, email: string): string { return name + email; }\n")
	renamed := parse(t, "a.ts", "export function f(user: string, mail: string): string { return user + mail; }\n")
	widened := parse(t, "a.ts", "export function f(name: string, email: string, x?: number): string { return name + email; }\n")
	retyped := parse(t, "a.ts", "export function f(name: number, email: string): string { return name + email; }\n")
	b, r, w, ty := base.Symbols[0], renamed.Symbols[0], widened.Symbols[0], retyped.Symbols[0]
	if b.SigHash != r.SigHash {
		t.Error("parameter rename must not change SigHash")
	}
	if b.BodyHash == r.BodyHash {
		t.Error("body edit must change BodyHash")
	}
	if b.SigHash == w.SigHash || b.SigHash == ty.SigHash {
		t.Error("added param or changed type must change SigHash")
	}
	if b.BodyHash != w.BodyHash {
		t.Error("signature-only change must keep BodyHash")
	}
}

func TestShinglesOnlyForLongBodies(t *testing.T) {
	long := `export function toSlug(input: string): string {
  const lower = input.trim().toLowerCase();
  const dashed = lower.replace(/[^a-z0-9]+/g, "-");
  const trimmed = dashed.replace(/^-+|-+$/g, "");
  return trimmed.length > 0 ? trimmed : "item";
}
export function tiny(): number { return 1; }
`
	f := parse(t, "a.ts", long)
	if len(sym(t, f, "toSlug").Shingles) == 0 {
		t.Error("long body should have shingles")
	}
	if len(sym(t, f, "tiny").Shingles) != 0 {
		t.Error("short body should not have shingles")
	}
}

func TestPartialAndJS(t *testing.T) {
	f := parse(t, "src/users.ts", "export function ok(): number { return 1; }\n\nexport const broken = ;\n")
	if !f.Partial {
		t.Error("syntax error should mark Partial")
	}
	sym(t, f, "ok")
	js := parse(t, "lib/x.js", "export function f(a, b = 1, ...r) { g(a); }\nclass K { m(x) {} }\n")
	ps := sym(t, js, "f").Signature.Params
	if len(ps) != 3 || ps[1].Shape() != "opt" || ps[2].Shape() != "rest" {
		t.Errorf("js params = %+v", ps)
	}
	sym(t, js, "K.m")
	if !New().Match("a.tsx") || !New().Match("a.mjs") || New().Match("a.py") {
		t.Error("Match")
	}
}
