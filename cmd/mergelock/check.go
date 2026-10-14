package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/rules"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

// checkRules is the rule set the check command runs; tests replace it.
var checkRules = rules.All

func runCheck(ctx context.Context, args []string, dir string, stdout, stderr io.Writer) (code int) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "json", "output format: json or text")
	noCache := fs.Bool("no-cache", false, "ignore and do not write the parse cache")
	positional, err := parseInterleaved(fs, args)
	if err != nil {
		return verdict.ExitOperational
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "mergelock:", err)
		if *format == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			enc.Encode(map[string]string{"error": err.Error()})
		}
		return verdict.ExitOperational
	}
	// A panic is an operational failure: exit 3, never 2 ("conflict").
	defer func() {
		if p := recover(); p != nil {
			code = fail(fmt.Errorf("internal error: %v", p))
		}
	}()
	if *format != "json" && *format != "text" {
		bad := *format
		*format = "text" // the format itself is invalid; report in plain text
		return fail(fmt.Errorf("unknown format %q (want json or text)", bad))
	}
	if len(positional) < 1 || len(positional) > 2 || slices.Contains(positional, "") {
		return fail(errors.New("usage: mergelock check <branchA> [branchB]"))
	}
	if err := gitx.RequireVersion(2, 38); err != nil {
		return fail(err)
	}
	a, b := positional[0], "" // empty b means the configured target
	if len(positional) == 2 {
		b = positional[1]
	}
	v, err := engine.CheckDir(ctx, dir, a, b, engine.Options{Rules: checkRules(), NoCache: *noCache})
	if err != nil {
		return fail(err)
	}
	if *format == "text" {
		fmt.Fprint(stdout, verdict.RenderText(v))
	} else {
		out, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
	}
	return verdict.ExitCode(v.Result)
}

// parseInterleaved lets flags appear before, between or after positional args.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}
