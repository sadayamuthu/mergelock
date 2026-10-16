# mergelock

[![CI](https://github.com/sadayamuthu/mergelock/actions/workflows/ci.yml/badge.svg)](https://github.com/sadayamuthu/mergelock/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sadayamuthu/mergelock?sort=semver)](https://github.com/sadayamuthu/mergelock/releases/latest)
[![Homebrew](https://img.shields.io/badge/brew-sadayamuthu%2Ftap%2Fmergelock-FBB040?logo=homebrew&logoColor=white)](https://github.com/sadayamuthu/homebrew-tap)
[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Fsadayamuthu%2Fmergelock-2496ED?logo=docker&logoColor=white)](https://github.com/sadayamuthu/mergelock/pkgs/container/mergelock)
[![deb/rpm](https://img.shields.io/badge/packages-.deb%20%7C%20.rpm-A81D33?logo=debian&logoColor=white)](https://github.com/sadayamuthu/mergelock/releases/latest)
[![MCP Registry](https://img.shields.io/badge/MCP%20Registry-io.github.sadayamuthu%2Fmergelock-6E56CF)](https://registry.modelcontextprotocol.io/v0/servers?search=io.github.sadayamuthu/mergelock)
[![Glama](https://glama.ai/mcp/servers/sadayamuthu/mergelock/badges/score.svg)](https://glama.ai/mcp/servers/sadayamuthu/mergelock)
[![License](https://img.shields.io/github/license/sadayamuthu/mergelock)](LICENSE)

A git-native merge gate. `mergelock check` compares two branches before
they meet and reports the conflicts git itself won't tell you about: a
function whose signature changed on one branch while the other branch
added calls to the old signature, a symbol deleted on one side but still
used on the other, the same function written twice.

![Two agent branches merge cleanly in git; mergelock reports that checkout.ts still calls login() with the old signature](demo/demo.gif)

It never touches your working tree. Everything is computed from commits
with `git merge-tree` and tree-sitter parsers, so it is safe to run in a
pre-push hook, in CI, or from an AI coding agent over MCP.

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

mergelock needs **git 2.38 or newer** (for `git merge-tree --write-tree`).

**Homebrew** (macOS and Linux):

```sh
brew install sadayamuthu/tap/mergelock
```

**Debian / Ubuntu** (`.deb`, amd64 and arm64; needs a distribution with git
2.38+, such as Debian 12 or Ubuntu 24.04):

```sh
VERSION=0.1.4   # latest release, without the v
curl -fLO "https://github.com/sadayamuthu/mergelock/releases/download/v${VERSION}/mergelock_${VERSION}_$(dpkg --print-architecture).deb"
sudo apt install "./mergelock_${VERSION}_$(dpkg --print-architecture).deb"
```

**Fedora / RHEL** (`.rpm`, x86_64 and aarch64):

```sh
VERSION=0.1.4
sudo dnf install "https://github.com/sadayamuthu/mergelock/releases/download/v${VERSION}/mergelock-${VERSION}-1.$(uname -m).rpm"
```

**Docker** (linux/amd64 and linux/arm64; mount the repository at `/repo`):

```sh
docker run --rm -v "$PWD":/repo ghcr.io/sadayamuthu/mergelock check my-branch main --format text
docker run -i --rm -v "$PWD":/repo ghcr.io/sadayamuthu/mergelock   # MCP server on stdio
```

**Prebuilt binary** (macOS and Linux, amd64 and arm64): download the archive
for your platform from the
[latest release](https://github.com/sadayamuthu/mergelock/releases/latest),
then put `mergelock` on your `PATH`:

```sh
tar -xzf mergelock_*_$(uname -s | tr A-Z a-z)_*.tar.gz
sudo mv mergelock /usr/local/bin/
```

On macOS, a binary from an archive downloaded in a browser is blocked by
Gatekeeper because it isn't signed yet (Homebrew installs aren't affected).
Clear the quarantine flag once:

```sh
xattr -d com.apple.quarantine /usr/local/bin/mergelock
```

**With Go** (needs Go 1.25 or newer and a C compiler, since cgo is used for
tree-sitter; on macOS install the Xcode command-line tools, on
Debian/Ubuntu `build-essential`):

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

## License

[Apache License 2.0](LICENSE)
