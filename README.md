# mergelock

[![CI](https://github.com/sadayamuthu/mergelock/actions/workflows/ci.yml/badge.svg)](https://github.com/sadayamuthu/mergelock/actions/workflows/ci.yml)

A git-native merge gate. `mergelock check` compares two branches before
they meet and reports the conflicts git itself won't tell you about: a
function whose signature changed on one branch while the other branch
added calls to the old signature, a symbol deleted on one side but still
used on the other, the same function written twice.

It never touches your working tree. Everything is computed from commits
with `git merge-tree` and tree-sitter parsers, so it is safe to run in a
pre-push hook, in CI, or from an AI coding agent over MCP.

```
$ mergelock check feat-a feat-b --format text
conflict: feat-a ↔ feat-b (base 4668b2e)

error    high     signature_caller  login
  changed in feat-a at src/auth.ts:1
  login(user: string): boolean → login(user: string, password: string): boolean
  call sites in feat-b:
    src/app.ts:4  login("admin")
  → Update 1 call site in feat-b to the new signature, or merge feat-b first and rebase feat-a.
```

## What it detects

| Rule | Reports | Severity |
|------|---------|----------|
| `textual` | Ordinary git content conflicts | error |
| `add_add` | Both branches add a file at the same path | error |
| `modify_delete` | One branch edits a file or symbol the other deletes, or still calls a deleted symbol | error |
| `signature_caller` | A signature changed on one branch; the other adds calls that no longer fit | error |
| `duplicate_definition` | Both branches add the same function, by name or by near-identical body | warning |

Semantic rules understand **TypeScript, JavaScript (incl. TSX/JSX) and
Python**. Files in other languages still get the textual, add/add and
modify/delete checks.

## Install

Requirements:

- **git 2.38 or newer** (for `git merge-tree --write-tree`)
- **Go 1.25 or newer** and a **C compiler** (cgo is used for tree-sitter;
  on macOS install the Xcode command-line tools, on Debian/Ubuntu `build-essential`)

```sh
go install github.com/sadayamuthu/mergelock/cmd/mergelock@latest
```

Or build from source:

```sh
git clone https://github.com/sadayamuthu/mergelock.git
cd mergelock
go build -ldflags "-X main.version=$(git describe --tags --always)" -o mergelock ./cmd/mergelock
```

Check it works:

```sh
mergelock version
```

## Quick start

Run inside any git repository:

```sh
# Is my branch safe to merge into main?
mergelock check my-feature main

# Compare against the configured target (main by default)
mergelock check my-feature

# Will two in-flight branches collide with each other?
mergelock check feat-a feat-b --format text
```

Exit codes make it easy to script:

| Code | Result |
|------|--------|
| 0 | `clean`: no findings |
| 1 | `warnings`: only warnings |
| 2 | `conflict`: at least one error |
| 3 | operational error (bad branch name, not a git repo, invalid config…) |

See **[USAGE.md](USAGE.md)** for the full command reference, the
`.mergelock.yml` config, the JSON output schema, CI and git-hook recipes,
and how to connect it to an AI agent over MCP.

## Development

```sh
go test ./...          # full suite, incl. fixture gate and 5,000-file perf test
go test -short ./...   # skips the perf test
go vet ./...
gofmt -l .
```

Rule behaviour is pinned by branch-pair fixtures in `testdata/fixtures/*.txtar`;
each one is a tiny repo with a base, two branches and the expected findings.
