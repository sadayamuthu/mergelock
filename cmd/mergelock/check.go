package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/sadayamuthu/mergelock/internal/config"
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/rules"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

func runCheck(ctx context.Context, args []string, dir string, stdout, stderr io.Writer) int {
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
	if *format != "json" && *format != "text" {
		bad := *format
		*format = "text" // the format itself is invalid; report in plain text
		return fail(fmt.Errorf("unknown format %q (want json or text)", bad))
	}
	if len(positional) < 1 || len(positional) > 2 {
		return fail(errors.New("usage: mergelock check <branchA> [branchB]"))
	}
	if err := gitx.RequireVersion(2, 38); err != nil {
		return fail(err)
	}
	repo, err := gitx.Open(dir)
	if err != nil {
		return fail(err)
	}
	cfg, err := config.Load(repo.Dir)
	if err != nil {
		return fail(err)
	}
	a, b := positional[0], cfg.Target
	if len(positional) == 2 {
		b = positional[1]
	}
	v, err := engine.Check(ctx, repo, a, b, engine.Options{Config: cfg, Rules: rules.All(), NoCache: *noCache})
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
