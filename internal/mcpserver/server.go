// Package mcpserver exposes the engine to coding agents over MCP. It adds
// transport only; every answer comes from engine.Check or git refs.
package mcpserver

import (
	"context"
	"encoding/json"
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
	mcp.AddTool(s, &mcp.Tool{
		Name:        "check",
		Description: "Check a branch against another for textual and semantic merge conflicts. Returns a mergelock verdict (schema v1) with findings an agent can act on.",
	}, d.check)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "claims",
		Description: "List live claims (declared work scopes) from refs/mergelock/claims, newest first, so you can pick an area no other agent is working in. Read-only in v0.1.",
	}, d.claims)
	return s
}

func (d Deps) check(ctx context.Context, _ *mcp.CallToolRequest, in CheckInput) (*mcp.CallToolResult, verdict.Verdict, error) {
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
