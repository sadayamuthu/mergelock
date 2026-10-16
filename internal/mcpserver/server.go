// Package mcpserver exposes the engine to coding agents over MCP. It adds
// transport only; every answer comes from engine.Check or git refs.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

type Deps struct {
	Dir     string // working directory the server was started in
	Version string
	Rules   []engine.Rule
	Now     func() time.Time
}

type CheckInput struct {
	Branch  string `json:"branch,omitempty" jsonschema:"branch to check; defaults to the current branch"`
	Against string `json:"against,omitempty" jsonschema:"branch to compare against; defaults to the configured target"`
}

type ClaimsInput struct{}

type Agent struct {
	Name    string `json:"name"`
	Session string `json:"session,omitempty"`
}

// Claim follows claim schema v1 from the product spec.
type Claim struct {
	Version    int      `json:"version"`
	Branch     string   `json:"branch"`
	Agent      *Agent   `json:"agent,omitempty"`
	Intent     string   `json:"intent,omitempty"`
	Paths      []string `json:"paths"`
	Symbols    []string `json:"symbols,omitempty"`
	Mode       string   `json:"mode,omitempty"`
	Created    string   `json:"created"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// ClaimsOutput wraps the list because MCP structured output must be an object.
type ClaimsOutput struct {
	Claims []Claim `json:"claims"`
}

func New(d Deps) *mcp.Server {
	if d.Now == nil {
		d.Now = time.Now
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "mergelock", Version: d.Version}, nil)
	// Both tools only read the repository: no branch, index or working-tree
	// change, and nothing outside it. check writes only its own parse cache.
	readOnly := func(title string) *mcp.ToolAnnotations {
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: new(bool)}
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:  "check",
		Title: "Check branches for merge conflicts",
		Description: "Simulate merging two committed branches and report textual and semantic conflicts, " +
			"including ones git merges cleanly: a changed function signature against new call sites, " +
			"a deleted symbol that is still used, duplicate definitions. Returns a verdict with result " +
			"clean, warnings or conflict, and per finding the file, line and a suggested_action to apply. " +
			"Call it after committing and before reporting work as done; fix every error finding, commit, " +
			"and call it again. Uncommitted changes are not seen. Use claims instead to see what other " +
			"agents are working on before you start.",
		Annotations: readOnly("Check branches for merge conflicts"),
	}, d.check)
	mcp.AddTool(s, &mcp.Tool{
		Name:  "claims",
		Title: "List live work claims",
		Description: "List live claims (work scopes other agents have declared: branch, paths, symbols, intent) " +
			"from refs/mergelock/claims, newest first; expired claims are left out. Call it before starting " +
			"a task to pick files and symbols no other agent is changing. It reports intent only; to find " +
			"actual conflicts between branches, use check. Read-only in v0.1: claims cannot be created here.",
		Annotations: readOnly("List live work claims"),
	}, d.claims)
	return s
}

func (d Deps) check(ctx context.Context, _ *mcp.CallToolRequest, in CheckInput) (_ *mcp.CallToolResult, _ verdict.Verdict, err error) {
	defer func() { // a panic must not take the server down; report a tool error
		if p := recover(); p != nil {
			err = fmt.Errorf("internal error: %v", p)
		}
	}()
	v, err := engine.CheckDir(ctx, d.Dir, in.Branch, in.Against, engine.Options{Rules: d.Rules})
	if err != nil {
		return nil, verdict.Verdict{}, err
	}
	return nil, *v, nil
}

func (d Deps) claims(ctx context.Context, _ *mcp.CallToolRequest, _ ClaimsInput) (*mcp.CallToolResult, ClaimsOutput, error) {
	repo, err := gitx.Open(d.Dir)
	if err != nil {
		return nil, ClaimsOutput{}, err
	}
	blobs, err := repo.RefBlobs("refs/mergelock/claims/")
	if err != nil {
		return nil, ClaimsOutput{}, err
	}
	now := d.Now()
	type live struct {
		claim   Claim
		created time.Time
	}
	var all []live
	for _, data := range blobs {
		var c Claim
		if json.Unmarshal(data, &c) != nil {
			continue
		}
		created, err := time.Parse(time.RFC3339, c.Created)
		if err != nil || !created.Add(time.Duration(c.TTLSeconds)*time.Second).After(now) {
			continue
		}
		all = append(all, live{c, created})
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].created.Equal(all[j].created) {
			return all[i].created.After(all[j].created)
		}
		return all[i].claim.Branch < all[j].claim.Branch
	})
	out := ClaimsOutput{Claims: []Claim{}}
	for _, l := range all {
		out.Claims = append(out.Claims, l.claim)
	}
	return nil, out, nil
}
