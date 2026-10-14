// Command mergelock is a git-native merge gate that detects semantic
// conflicts between branches.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `usage: mergelock <command> [arguments]

commands:
  check <branchA> [branchB] [--format json|text] [--no-cache]
        compare two branches; branchB defaults to the configured target
  version
        print version information
`

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mergelock:", err)
		os.Exit(verdict.ExitOperational)
	}
	os.Exit(run(context.Background(), os.Args[1:], dir, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, dir string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return verdict.ExitOperational
	}
	switch args[0] {
	case "check":
		return runCheck(ctx, args[1:], dir, stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "mergelock %s\n", version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "mergelock: unknown command %q\n\n%s", args[0], usage)
	return verdict.ExitOperational
}
