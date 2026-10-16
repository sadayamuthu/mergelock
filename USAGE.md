# Using mergelock

- [Commands](#commands)
- [Configuration](#configuration)
- [Reading the output](#reading-the-output)
- [In CI](#in-ci)
- [As a git hook](#as-a-git-hook)
- [With AI agents (MCP)](#with-ai-agents-mcp)
- [Realtime with parallel agents](#realtime-with-parallel-agents)
- [Cache](#cache)
- [Limits](#limits)

## Commands

```
mergelock check <branchA> [branchB] [--format json|text] [--no-cache]
mergelock serve
mergelock version
```

### `check`

Compares `branchA` with `branchB` as if they were merged at their merge
base. Any revision git understands works: branch names, `HEAD`,
`origin/main`, tags or commit SHAs. When `branchB` is omitted, the
`target` from `.mergelock.yml` is used (default `main`).

| Flag | Default | Meaning |
|------|---------|---------|
| `--format` | `json` | `json` for machines, `text` for people |
| `--no-cache` | off | Neither read nor write the parse cache |

Flags can go before, between or after the branch names. Run it from
anywhere inside the repository. Your working tree, index and branches are
never modified.

### `serve`

Runs an MCP server over stdio. See [With AI agents](#with-ai-agents-mcp).

### `version`

Prints the mergelock version, the git version it found and the language
adapters it carries:

```
mergelock v0.1.0
git 2.50
adapters: typescript@1 python@1
```

## Configuration

Optional. Put `.mergelock.yml` at the repository root:

```yaml
# Branch to compare against when check gets only one branch.
target: main

rules:
  # Turn a rule off with `off` or `false`.
  textual: on
  add_add: on
  modify_delete: on
  signature_caller: on
  duplicate_definition:
    # Body similarity (0–1) above which two new functions with different
    # names count as duplicates. Default 0.85.
    similarity: 0.85
```

Rule ids that are reserved for later versions (`interface_drift`,
`registry_hotspot`, `test_contract`, `claim_drift`) are accepted and have
no effect yet. So are the keys `test`, `block_on`, `min_confidence`,
`strict_claims` and `lock_timeout_seconds`.

Unknown keys, unknown rule ids and an out-of-range `similarity` are
rejected with exit code 3, so a typo never silently disables a check.

## Reading the output

`--format json` (the default) prints one verdict object:

```json
{
  "version": 1,
  "base": "4668b2e52cbb2bb42f059b8e5220cd2b9f740566",
  "branches": ["feat-a", "feat-b"],
  "result": "conflict",
  "coverage": {
    "languages": ["typescript"],
    "unsupported": [],
    "partial": []
  },
  "findings": [
    {
      "class": "signature_caller",
      "severity": "error",
      "confidence": "high",
      "symbol": "login",
      "changed_in": "feat-a",
      "changed_at": { "file": "src/auth.ts", "line": 1 },
      "old_signature": "login(user: string): boolean",
      "new_signature": "login(user: string, password: string): boolean",
      "used_in": "feat-b",
      "call_sites": [
        { "file": "src/app.ts", "line": 4, "text": "login(\"admin\")" }
      ],
      "suggested_action": "Update 1 call site in feat-b to the new signature, or merge feat-b first and rebase feat-a."
    }
  ],
  "stats": { "symbols_compared": 3, "rules_run": 5, "duration_ms": 95 }
}
```

Key fields:

- **`result`**: `clean`, `warnings` or `conflict`. Matches the exit code (0, 1, 2).
- **`findings[].class`**: the rule that fired. `kind` refines it where a
  rule has several cases (for example `deleted_symbol_referenced`).
- **`findings[].confidence`**:
  - `certain`: reported by git or an exact symbol match.
  - `high`: the call was resolved through imports to exactly this symbol.
  - `medium`: a likely match the parser couldn't prove, such as a method
    call on a receiver of unknown type. Python findings are capped at medium.
- **`findings[].suggested_action`**: a one-line fix, written to be acted
  on by a person or an agent.
- **`coverage.unsupported`**: changed files in languages without a semantic
  adapter. Only git-level rules ran on them.
- **`coverage.partial`**: files the parser could only partly read. Findings
  that depend on them are downgraded rather than dropped.

On an operational error, JSON mode prints `{"error": "..."}` and exits 3.

## In CI

Fail a pull request that would conflict with its base branch:

```yaml
# .github/workflows/mergelock.yml
name: mergelock
on: pull_request

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0          # mergelock needs the merge base
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go install github.com/sadayamuthu/mergelock/cmd/mergelock@latest
      - name: Check against base branch
        run: mergelock check "$HEAD" "origin/$BASE" --format text
        env:
          HEAD: ${{ github.event.pull_request.head.sha }}
          BASE: ${{ github.base_ref }}
```

Use the PR's head commit, not `HEAD`: on `pull_request` events the
checkout is a temporary merge commit that already contains the base branch.

The job fails on exit 1 (warnings) as well as 2 (conflict). To fail on
conflicts only, replace the `run` line with:

```sh
mergelock check "$HEAD" "origin/$BASE" --format text || [ $? -eq 1 ]
```

## As a git hook

Block a push that would conflict with `main`. Save as `.git/hooks/pre-push`
and make it executable (`chmod +x .git/hooks/pre-push`):

```sh
#!/bin/sh
git fetch -q origin main
mergelock check HEAD origin/main --format text
[ $? -le 1 ]   # allow warnings, block conflicts and errors
```

## With AI agents (MCP)

`mergelock serve` exposes two tools over the Model Context Protocol:

| Tool | Input | Returns |
|------|-------|---------|
| `check` | `branch` (default: current branch), `against` (default: configured target) | The same verdict as `check --format json` |
| `claims` | none | `{"claims": [...]}`: live work-scope claims from `refs/mergelock/claims/`, newest first (read-only in v0.1) |

The server works on the repository it is started in.

**Claude Code** (run in the repository):

```sh
claude mcp add mergelock -- mergelock serve
```

**Cursor** (`.cursor/mcp.json`) or any client that uses the `mcpServers` format:

```json
{
  "mcpServers": {
    "mergelock": {
      "command": "mergelock",
      "args": ["serve"]
    }
  }
}
```

**Codex** (`~/.codex/config.toml`):

```toml
[mcp_servers.mergelock]
command = "mergelock"
args = ["serve"]
```

Then add this to the agent's standing instructions (`CLAUDE.md`,
`.cursor/rules`, `AGENTS.md`):

> After each commit, and before saying you're done, call the mergelock
> `check` tool. Fix every finding with severity `error`, commit, and check
> again.

## Realtime with parallel agents

mergelock compares **commits**, not uncommitted files. To get continuous
feedback while agents work, give each agent its own branch or worktree,
have it commit often, and run the check at the moments it makes progress.

| Recipe | Catches | When |
|--------|---------|------|
| [MCP + instructions](#with-ai-agents-mcp) | agent's branch vs `main` | while the agent works |
| [Claude Code Stop hook](#claude-code-block-finishing-on-a-conflict) | agent's branch vs `main` | before Claude can finish (enforced) |
| [post-commit hook](#every-commit-check-against-other-agents) | agent vs agent | on every commit |
| [Watch script](#watch-all-branches) | every pair of branches | continuously, for you |
| [CI job](#in-ci) | pull request vs base | final gate |

### Claude Code: block finishing on a conflict

A `Stop` hook runs whenever Claude tries to finish. Exit code 2 keeps
Claude working and shows it what the hook printed to stderr. In
`.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "mergelock check HEAD main --format text >&2; [ $? -eq 2 ] && exit 2 || exit 0"
          }
        ]
      }
    ]
  }
}
```

Claude then sees findings like *"Update 1 call site in HEAD to the new
signature"* and fixes them before stopping. Warnings don't block.

### Every commit: check against other agents

Name agent branches `agent/*` and save this as `.git/hooks/post-commit`
(`chmod +x`). Worktrees share the repository's hooks, so every agent's
commits run it:

```bash
#!/bin/bash
me=$(git branch --show-current)
for other in $(git for-each-ref --format='%(refname:short)' refs/heads/agent/); do
  [ "$other" = "$me" ] && continue
  out=$(mergelock check "$me" "$other" --format text)
  [ $? -ge 1 ] && printf '⚠ mergelock: %s vs %s\n%s\n' "$me" "$other" "$out" >&2
done
exit 0
```

The commit always succeeds. The warning appears in the agent's command
output, which agents normally read and act on.

### Watch all branches

Run in a spare terminal while agents work:

```bash
#!/bin/bash
# mergelock-watch.sh: recheck every agent branch every 10 seconds
while true; do
  clear; date
  branches=$(git for-each-ref --format='%(refname:short)' refs/heads/agent/)
  for a in $branches; do
    mergelock check "$a" main --format text | head -1
    for b in $branches; do
      [[ "$a" < "$b" ]] && mergelock check "$a" "$b" --format text | head -1
    done
  done
  sleep 10
done
```

Output:

```
clean: agent/a ↔ main (base 4668b2e)
conflict: agent/a ↔ agent/b (base 4668b2e)
clean: agent/b ↔ main (base 4668b2e)
```

Run `mergelock check agent/a agent/b --format text` for the details of a
conflicting pair.

## Cache

Parsed files are cached by git blob id under
`<git common dir>/mergelock/cache/v1`, which is `.git/mergelock/cache/v1`
in a normal checkout. All worktrees of a repository share it. Warm runs
on a 5,000-file repository take about 125 ms.

The cache is safe to delete at any time. `--no-cache` bypasses it for a
single run, and results are identical with or without it.

## Limits

v0.1 deliberately reports only what it can resolve. Known gaps:

- Calls through typed class fields (`this.svc.login()` where
  `svc: AuthService`) are not followed.
- Chained receivers (`svc.repo.save()`) are not reported.
- In Python, a receiver assigned from a factory call
  (`repo = get_repo(); repo.save()`) is not followed.
- `duplicate_definition` compares top-level functions, not methods, and
  ignores tests, fixtures, examples and docs (`test/`, `__tests__/`,
  `*.spec.ts`, `test_*.py`, `examples/`, `docs/` and similar), where the same
  conventional names recur by design. It also skips generated and vendored
  code in directories such as `vendor/`, `node_modules/`, `dist/`, `build/`,
  `generated/` and `__generated__/`, and files such as `*.d.ts`,
  `*.generated.ts`, `*.gen.ts`, `*_pb2.py`, `*_pb2_grpc.py` and `*.min.js`.
  Each duplicated name is one finding listing every place it was added.
- A symbol-level delete-vs-edit (`deleted_symbol_modified`) is reported only
  when git also conflicts on that file. If git merges the file cleanly, the
  edit survived (for example the function was inlined), so it isn't reported.
- Semantic rules cover TypeScript, JavaScript and Python only.
