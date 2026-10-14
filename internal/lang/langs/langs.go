// Package langs assembles the default adapter registry.
package langs

import (
	"github.com/sadayamuthu/mergelock/internal/lang"
	"github.com/sadayamuthu/mergelock/internal/lang/typescript"
)

// Default returns the registry used by the CLI and MCP server.
func Default() *lang.Registry {
	return lang.NewRegistry(typescript.New())
}
