// Package bench generates large synthetic repositories for performance tests.
package bench

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/gittest"
)

func filePath(i int) string { return fmt.Sprintf("src/m%02d/f%04d.ts", i/100, i) }

// rel returns the import specifier from file `from` to file `to`.
func rel(from, to int) string {
	if from/100 == to/100 {
		return fmt.Sprintf("./f%04d", to)
	}
	return fmt.Sprintf("../m%02d/f%04d", to/100, to)
}

// body renders file i. widened adds a required parameter to f<i>_0; callee >= 0
// makes the file import and call f<callee>_0 with the old two-argument shape.
func body(i int, widened bool, callee int) string {
	var b strings.Builder
	if i > 0 {
		fmt.Fprintf(&b, "import { f%d_1 } from %q;\n", i-1, rel(i, i-1))
	}
	if callee >= 0 {
		fmt.Fprintf(&b, "import { f%d_0 } from %q;\n", callee, rel(i, callee))
	}
	if widened {
		fmt.Fprintf(&b, "export function f%d_0(a: number, b: number, c: number): number {\n  return a + b + c;\n}\n", i)
	} else {
		fmt.Fprintf(&b, "export function f%d_0(a: number, b: number): number {\n  return a + b;\n}\n", i)
	}
	fmt.Fprintf(&b, "export function f%d_1(x: string): string {\n  return x.trim();\n}\n", i)
	fmt.Fprintf(&b, "export class C%d {\n  run(n: number): number {\n    return n * %d;\n  }\n}\n", i, i)
	if callee >= 0 {
		fmt.Fprintf(&b, "export function use%d(): number {\n  return f%d_0(1, 2);\n}\n", i, callee)
	}
	return b.String()
}

// Generate builds n TypeScript files on main, branch a widening the first
// `changed` signatures and branch b adding callers of them in the next `changed` files.
func Generate(t testing.TB, n, changed int) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	// Auto-gc spawns a detached git that races with TempDir cleanup.
	r.Git("config", "gc.auto", "0")
	for i := 0; i < n; i++ {
		r.Write(filePath(i), body(i, false, -1))
	}
	r.Commit("base")
	r.Branch("a")
	for i := 0; i < changed; i++ {
		r.Write(filePath(i), body(i, true, -1))
	}
	r.Commit("a")
	r.Checkout("main")
	r.Branch("b")
	for i := changed; i < 2*changed; i++ {
		r.Write(filePath(i), body(i, false, i-changed))
	}
	r.Commit("b")
	r.Checkout("main")
	return r
}
