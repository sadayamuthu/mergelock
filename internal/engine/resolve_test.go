package engine

import (
	"testing"

	"github.com/sadayamuthu/mergelock/internal/gittest"
	"github.com/sadayamuthu/mergelock/internal/model"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

func resolveFixture(t *testing.T) (*RuleContext, *Side) {
	t.Helper()
	r := gittest.New(t)
	r.Write("src/auth/service.ts", `export class AuthService {
  login(a: string, b: string): string { return a + b; }
}
export function login(a: string): string { return a; }
`)
	r.Write("src/auth/index.ts", "export * from \"./service\";\n")
	r.Write("src/def.ts", "export default function makeDef(x: number): number { return x; }\n")
	r.Write("src/users.ts", `import { AuthService, login } from "./auth";
import * as svcmod from "./auth/service";
import makeDef from "./def";
export class Users {
  create(svc: AuthService, other) {
    login("x");
    svc.login("a", "b");
    other.login("a", "b");
    svcmod.login("y");
    new AuthService();
    makeDef(1);
    this.helper();
  }
  helper() {}
}
`)
	r.Write("src/other.ts", "export function f(client) { return client.login(\"a\", \"b\"); }\n")
	head := r.Commit("base")
	l, _ := newLoader(t, r.Dir, true)
	rc := &RuleContext{loader: l, reg: l.reg}
	return rc, &Side{Branch: "main", Commit: head}
}

func refByText(t *testing.T, rc *RuleContext, side *Side, file, text string) model.Reference {
	t.Helper()
	f, err := rc.loader.FactsAt(side.Commit, file)
	if err != nil || f == nil {
		t.Fatalf("FactsAt(%s) = %v", file, err)
	}
	for _, r := range f.References {
		if r.Callee.Text() == text {
			return r
		}
	}
	t.Fatalf("no reference %q in %s", text, file)
	return model.Reference{}
}

func TestResolveRef(t *testing.T) {
	rc, side := resolveFixture(t)
	res := &resolver{l: rc.loader, reg: rc.reg}
	cases := map[string]string{
		"login":           "src/auth/service.ts::login",
		"svc.login":       "src/auth/service.ts::AuthService.login",
		"svcmod.login":    "src/auth/service.ts::login",
		"new AuthService": "src/auth/service.ts::AuthService",
		"makeDef":         "src/def.ts::makeDef",
		"this.helper":     "src/users.ts::Users.helper",
	}
	for text, want := range cases {
		got, ok := res.resolveRef(side.Commit, refByText(t, rc, side, "src/users.ts", text))
		if !ok || got != want {
			t.Errorf("resolveRef(%s) = %q, %v; want %q", text, got, ok, want)
		}
	}
	if got, ok := res.resolveRef(side.Commit, refByText(t, rc, side, "src/users.ts", "other.login")); ok {
		t.Errorf("unknown receiver must not bind exactly, got %q", got)
	}
}

func TestMatches(t *testing.T) {
	rc, side := resolveFixture(t)
	svcFacts, _ := rc.loader.FactsAt(side.Commit, "src/auth/service.ts")
	var method, fn model.Symbol
	for _, s := range svcFacts.Symbols {
		switch s.Name {
		case "AuthService.login":
			method = s
		case "login":
			fn = s
		}
	}
	check := func(file, text string, sym model.Symbol, wantConf verdict.Confidence, wantOK bool) {
		t.Helper()
		c, ok := rc.Matches(side, refByText(t, rc, side, file, text), sym)
		if ok != wantOK || (ok && c != wantConf) {
			t.Errorf("Matches(%s, %s) = %s, %v; want %s, %v", text, sym.Name, c, ok, wantConf, wantOK)
		}
	}
	check("src/users.ts", "svc.login", method, verdict.ConfidenceHigh, true)
	check("src/users.ts", "svc.login", fn, "", false)
	check("src/users.ts", "login", fn, verdict.ConfidenceHigh, true)
	check("src/users.ts", "other.login", method, verdict.ConfidenceMedium, true)
	check("src/users.ts", "other.login", fn, "", false)
	check("src/other.ts", "client.login", method, "", false)
}
