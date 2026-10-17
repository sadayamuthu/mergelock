# Contributing to mergelock

Thanks for helping. mergelock is only useful if its findings can be trusted,
so most of this guide is about keeping it precise: every change in behaviour
comes with a fixture that proves it.

- [Ways to help](#ways-to-help)
- [Reporting a wrong or missed finding](#reporting-a-wrong-or-missed-finding)
- [Development setup](#development-setup)
- [Fixtures](#fixtures)
- [Code layout](#code-layout)
- [Adding a rule](#adding-a-rule)
- [Adding a language](#adding-a-language)
- [Pull requests](#pull-requests)
- [Security](#security)
- [License](#license)

## Ways to help

- **Report a false positive or a missed conflict.** This is the most useful
  contribution. See the next section.
- **Run it on your repository** and tell us what it found, in an issue or a
  discussion. Real-world results guide what to fix next.
- **Pick up an issue** labelled
  [`good first issue`](https://github.com/sadayamuthu/mergelock/labels/good%20first%20issue)
  or [`help wanted`](https://github.com/sadayamuthu/mergelock/labels/help%20wanted).
- **Close a known gap** from the [Limits](USAGE.md#limits) section of the
  usage guide, or add a language adapter.

For anything larger than a small fix, open an issue first so we can agree on
the approach before you write the code.

## Reporting a wrong or missed finding

Please include:

1. `mergelock version` output.
2. The command you ran and its `--format json` output.
3. What you expected instead, and why.
4. A minimal reproduction, ideally as a [fixture](#fixtures): a few small
   files for the base and each branch. A fixture is the fastest way to get a
   fix, because it becomes the test.

If the code is private, reduce it to a made-up example that still shows the
problem.

## Development setup

You need:

- **Go 1.25** or newer
- **A C compiler**, because the tree-sitter parsers use cgo (Xcode command-line
  tools on macOS, `build-essential` on Debian/Ubuntu)
- **git 2.38** or newer (for `git merge-tree --write-tree`)

```sh
git clone https://github.com/sadayamuthu/mergelock.git
cd mergelock
go build -o mergelock ./cmd/mergelock
./mergelock version
```

Before pushing, run what CI runs:

```sh
gofmt -l .                # must print nothing
go vet ./...
go mod tidy && git diff --exit-code go.mod go.sum
go test ./...             # full suite, including the 5,000-file perf test
go test -race -short ./...
```

`go test -short ./...` skips the performance test for a faster loop.

## Fixtures

Rule behaviour is pinned by branch-pair fixtures in
[`testdata/fixtures/`](testdata/fixtures). Each `.txtar` file describes a tiny
repository: a base commit, branch `a` and branch `b` built on it, and the
verdict `mergelock check a b` must return.

```
a adds a required parameter to login; b adds a new call with the old arguments.
-- base/src/auth.ts --
export function login(user: string): boolean {
  return user.length > 0;
}
-- a/src/auth.ts --
export function login(user: string, password: string): boolean {
  return user.length > 0 && password.length > 0;
}
-- b/src/app.ts --
import { login } from "./auth";

export const ok = login("admin");
-- expected.json --
{"result": "conflict", "findings": [{"class": "signature_caller", "symbol": "login", "min_confidence": "high"}]}
```

- The first line is a description of the case.
- `-- base/<path> --`, `-- a/<path> --` and `-- b/<path> --` add or replace a
  file on that side. Files not mentioned on a branch are unchanged from base.
- `-- a.deleted --` and `-- b.deleted --` list paths (one per line) that the
  branch deletes.
- `-- expected.json --` holds the expected `result` (`clean`, `warnings` or
  `conflict`) and `findings`. Each finding is matched on `class` and,
  if given, `kind`, `symbol`, `renamed_to`, `min_confidence` and
  `max_confidence`. Any finding the run reports that isn't listed fails the
  fixture.

**Naming:** `<lang>-<rule>-<case>.txtar`, for example `ts-sig-barrel.txtar`.
Fixtures that must stay clean start with `<lang>-neg-`.

**The gate:** `TestFixtures` requires precision and recall of 1.00 per rule
class across the whole suite, so a change that fixes one fixture but breaks
another fails. `TestSuiteComposition` keeps the suite balanced (at least 30
fixtures, at least 8 of them negative).

Run one fixture while you work on it:

```sh
go test ./internal/fixtures -run 'TestFixtures/ts-sig-barrel' -v
```

**Every behaviour change needs a fixture.** For a bug fix, add a fixture that
fails before your change and passes after it. For a false positive, add a
`-neg-` fixture.

## Code layout

| Path | What it does |
|------|--------------|
| `cmd/mergelock` | CLI entry point: `check`, `serve`, `version` |
| `internal/gitx` | git plumbing: merge base, `merge-tree`, blobs, refs |
| `internal/engine` | Loads changed files, runs the rules, builds the verdict |
| `internal/rules` | One file per rule (`textual`, `add_add`, `modify_delete`, `signature_caller`, `duplicate_definition`) |
| `internal/lang` | Language adapter interface and the tree-sitter helpers |
| `internal/lang/typescript`, `internal/lang/python` | Language adapters: symbols, references, import resolution |
| `internal/verdict` | The JSON verdict schema and the text renderer |
| `internal/config` | `.mergelock.yml` loading and validation |
| `internal/cache` | Parse cache keyed by git blob id |
| `internal/mcpserver` | MCP server and its `check` and `claims` tools |
| `internal/fixtures` | Fixture parser and the precision/recall gate |
| `internal/bench` | 5,000-file performance test |

## Adding a rule

1. Implement `engine.Rule` (`ID()` and `Run(*engine.RuleContext)`) in a new
   file under `internal/rules/`, and add it to `rules.All()`.
2. Every finding needs a `severity`, a `confidence` you can justify (see
   [Reading the output](USAGE.md#reading-the-output)) and a one-line
   `suggested_action` that a person or an agent can act on.
3. Add positive and `-neg-` fixtures for it.
4. Register its id in `internal/config` so it can be turned off in
   `.mergelock.yml`, and document it in the README and USAGE.md.

When the rule can't prove a finding, prefer reporting it at a lower
confidence, or not at all, over a confident false positive.

## Adding a language

A language adapter implements the interface in
[`internal/lang/lang.go`](internal/lang/lang.go): it parses a file with
tree-sitter and extracts symbols (with signatures), references and imports,
and resolves imports to repository paths. Use the TypeScript and Python
adapters as models, register the new one in `internal/lang/langs`, and add
fixtures for at least `signature_caller`, `modify_delete` and one negative
case.

Open an issue before starting, so two people don't build the same adapter.

## Pull requests

- Keep each PR to one change; small PRs get reviewed faster.
- Use the commit message style in the history:
  `type(scope): summary`, for example
  `fix(rules): skip generated files in duplicate_definition`. Types in use:
  `feat`, `fix`, `test`, `docs`, `build`, `chore`.
- CI must pass on Linux and macOS.
- Update README.md or USAGE.md when you change behaviour a user can see.

## Security

Please don't report security problems in a public issue. Use
[**Report a vulnerability**](https://github.com/sadayamuthu/mergelock/security/advisories/new)
on the repository's Security tab instead. See [SECURITY.md](SECURITY.md) for
what to include and what's in scope.

## License

mergelock is licensed under the [Apache License 2.0](LICENSE). By
contributing, you agree that your contributions are licensed under the same
license.
