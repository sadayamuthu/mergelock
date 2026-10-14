package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/lang/langs"
	"github.com/sadayamuthu/mergelock/internal/mcpserver"
	"github.com/sadayamuthu/mergelock/internal/rules"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

func runServe(ctx context.Context, dir string, stderr io.Writer) int {
	if err := gitx.RequireVersion(2, 38); err != nil {
		fmt.Fprintln(stderr, "mergelock:", err)
		return verdict.ExitOperational
	}
	s := mcpserver.New(mcpserver.Deps{Dir: dir, Version: version, Rules: rules.All()})
	if err := s.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(stderr, "mergelock:", err)
		return verdict.ExitOperational
	}
	return 0
}

func runVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "mergelock %s\n", version)
	if maj, min, err := gitx.Version(); err == nil {
		fmt.Fprintf(stdout, "git %d.%d\n", maj, min)
	}
	var names []string
	for _, a := range langs.Default().All() {
		names = append(names, a.Name()+"@"+a.Version())
	}
	fmt.Fprintf(stdout, "adapters: %s\n", strings.Join(names, " "))
	return 0
}
