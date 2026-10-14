package python

import (
	"testing"

	"github.com/sadayamuthu/mergelock/internal/model"
)

const sample = `import app.auth
import app.auth as au
from . import sibling
from .service import AuthService, login as do_login
from ..pkg.mod import *

class AuthService(Base):
    def __init__(self, db, opt=None):
        self.db = db

    @staticmethod
    def make(a, *args, key, flag=False, **kw) -> "AuthService":
        return AuthService(db)

    def login(self, name: str, email: str = "") -> Session:
        return self.check(name)

def helper(a, b=1):
    svc = AuthService(db)
    svc.login("a", email="b")
    app.auth.login(*xs, **kw)
    au.AuthService.make(1)
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
	t.Fatalf("symbol %q not in %+v", name, f.Symbols)
	return model.Symbol{}
}

func shapes(ps []model.Param) string {
	s := ""
	for _, p := range ps {
		s += p.Name + ":" + p.Shape() + " "
	}
	return s
}

func TestSymbols(t *testing.T) {
	f := parse(t, "app/auth.py", sample)
	if f.Lang != "python" || f.Partial || len(f.Symbols) != 5 {
		t.Fatalf("facts = %+v", f)
	}
	if got := shapes(sym(t, f, "AuthService.__init__").Signature.Params); got != "db:req opt:opt " {
		t.Errorf("__init__ params = %q", got)
	}
	if got := shapes(sym(t, f, "AuthService.make").Signature.Params); got != "a:req args:rest key:kw flag:kwopt kw:kwrest " {
		t.Errorf("make params = %q", got)
	}
	login := sym(t, f, "AuthService.login")
	if got := shapes(login.Signature.Params); got != "name:req email:opt " {
		t.Errorf("login params = %q", got)
	}
	if login.Signature.Text != `login(self, name: str, email: str = "") -> Session` || login.Kind != model.KindMethod || !login.Exported {
		t.Errorf("login = %+v", login)
	}
	if sym(t, f, "helper").Kind != model.KindFunction || sym(t, f, "AuthService").Kind != model.KindClass {
		t.Error("kinds")
	}
	renamed := parse(t, "a.py", "def f(name, mail=\"\"):\n    return name\n")
	orig := parse(t, "a.py", "def f(name, email=\"\"):\n    return name\n")
	if renamed.Symbols[0].SigHash == orig.Symbols[0].SigHash {
		t.Error("Python SigHash must include parameter names")
	}
}

func TestImports(t *testing.T) {
	f := parse(t, "app/x/auth.py", sample)
	want := []model.Import{
		{Local: "app.auth", Imported: "*", Spec: "app.auth"},
		{Local: "au", Imported: "*", Spec: "app.auth"},
		{Local: "sibling", Imported: "sibling", Spec: "."},
		{Local: "AuthService", Imported: "AuthService", Spec: ".service"},
		{Local: "do_login", Imported: "login", Spec: ".service"},
		{Local: "", Imported: "*", Spec: "..pkg.mod"},
	}
	if len(f.Imports) != len(want) {
		t.Fatalf("Imports = %+v", f.Imports)
	}
	for i := range want {
		if f.Imports[i] != want[i] {
			t.Errorf("Imports[%d] = %+v, want %+v", i, f.Imports[i], want[i])
		}
	}
	if len(f.ReExports) != 1 || f.ReExports[0] != (model.ReExport{Name: "*", Imported: "*", Spec: "..pkg.mod"}) {
		t.Errorf("ReExports = %+v", f.ReExports)
	}
	init := parse(t, "app/auth/__init__.py", "from .service import login\n")
	if len(init.ReExports) != 1 || init.ReExports[0] != (model.ReExport{Name: "login", Imported: "login", Spec: ".service"}) {
		t.Errorf("__init__ ReExports = %+v", init.ReExports)
	}
}

func TestReferences(t *testing.T) {
	f := parse(t, "app/auth.py", sample)
	type want struct {
		from, recv string
		args       int
		spread     bool
		kws        string
	}
	cases := map[string]want{
		"this.check":          {"AuthService.login", "", 1, false, ""},
		"svc.login":           {"helper", "AuthService", 1, false, "email"},
		"app.auth.login":      {"helper", "", 0, true, ""},
		"au.AuthService.make": {"helper", "", 1, false, ""},
	}
	found := map[string]bool{}
	for _, r := range f.References {
		w, ok := cases[r.Callee.Text()]
		if !ok {
			continue
		}
		found[r.Callee.Text()] = true
		kws := ""
		for _, k := range r.Keywords {
			kws += k
		}
		if model.ScopeName(r.FromKey) != w.from || r.Callee.ReceiverType != w.recv || r.ArgCount != w.args || r.Spread != w.spread || kws != w.kws {
			t.Errorf("%s = %+v", r.Callee.Text(), r)
		}
	}
	for text := range cases {
		if !found[text] {
			t.Errorf("reference %q not found", text)
		}
	}
	var ctorCalls int
	for _, r := range f.References {
		if r.Callee.Text() == "AuthService" && r.ArgCount == 1 {
			ctorCalls++
		}
	}
	if ctorCalls != 2 {
		t.Errorf("AuthService(db) calls = %d, want 2", ctorCalls)
	}
}

type mapFS map[string]bool

func (m mapFS) Exists(p string) bool       { return m[p] }
func (m mapFS) Read(string) ([]byte, bool) { return nil, false }

func TestResolveModule(t *testing.T) {
	a := New()
	cases := []struct {
		fs         mapFS
		from, spec string
		want       string
		ok         bool
	}{
		{mapFS{"app/auth.py": true}, "app/users.py", "app.auth", "app/auth.py", true},
		{mapFS{"app/auth/__init__.py": true}, "app/users.py", "app.auth", "app/auth/__init__.py", true},
		{mapFS{"src/app/auth.py": true}, "src/app/users.py", "app.auth", "src/app/auth.py", true},
		{mapFS{"app/auth/service.py": true}, "app/auth/__init__.py", ".service", "app/auth/service.py", true},
		{mapFS{"app/pkg/mod.py": true}, "app/x/y.py", "..pkg.mod", "app/pkg/mod.py", true},
		{mapFS{"app/__init__.py": true}, "app/users.py", ".", "app/__init__.py", true},
		{mapFS{}, "app/users.py", "requests", "", false},
	}
	for _, c := range cases {
		got, ok := a.ResolveModule(c.from, c.spec, c.fs)
		if got != c.want || ok != c.ok {
			t.Errorf("ResolveModule(%q, %q) = %q, %v; want %q, %v", c.from, c.spec, got, ok, c.want, c.ok)
		}
	}
	if s, _ := a.SubmoduleSpec(".", "mod"); s != ".mod" {
		t.Errorf("SubmoduleSpec(., mod) = %q", s)
	}
	if s, _ := a.SubmoduleSpec("pkg", "mod"); s != "pkg.mod" {
		t.Errorf("SubmoduleSpec(pkg, mod) = %q", s)
	}
}
