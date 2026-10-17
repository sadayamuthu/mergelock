# Security Policy

## Reporting a vulnerability

Please don't report security problems in a public issue, pull request or
discussion. Use
[**Report a vulnerability**](https://github.com/sadayamuthu/mergelock/security/advisories/new)
on the repository's Security tab instead. Only the maintainer can see the
report.

Include what you can of:

- the output of `mergelock version` and your OS
- how mergelock was run: the CLI, the MCP server (`mergelock serve`), the
  Docker image, or the GitHub Action
- steps to reproduce, ideally a small repository or a script that builds one
- what happened and what you expected

You'll get a reply when the report has been read, and again when a fix is
ready. A fix ships as a patch release; the advisory names you unless you'd
rather it didn't.

## Supported versions

mergelock is pre-1.0. Fixes land in the latest release only; please upgrade
before reporting.

## What counts

mergelock reads branches you may not trust, such as a pull request from a
fork, and parses their source code. It never writes to your working tree;
the only thing it writes is a parse cache under `.git/mergelock/`. Anything
that breaks one of these promises is a security issue:

- **Reading is all it does.** A branch, file, path or `.mergelock.yml` that
  makes mergelock run a command, write outside its cache, change the
  repository, or make a network call is in scope.
- **Parsing is safe.** Source that crashes the parser in a way that corrupts
  memory, or that hangs or exhausts memory far beyond the size of the input,
  is in scope.
- **The MCP server answers only what it's asked.** A tool argument that
  reads files outside the repository, runs git with options of the caller's
  choosing, or returns content other than the verdict is in scope.
- **The GitHub Action.** Script injection through branch names, file names
  or finding text in the job summary or annotations is in scope.

A wrong verdict, such as a conflict mergelock missed, is a bug, not a
vulnerability. Report it in a normal issue; see
[CONTRIBUTING.md](CONTRIBUTING.md).
