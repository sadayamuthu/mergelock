package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sadayamuthu/mergelock/internal/gittest"
	"github.com/sadayamuthu/mergelock/internal/rules"
)

func repoWithConflictAndClaims(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write("notes.txt", "status: draft\n")
	r.Commit("base")
	r.Branch("a")
	r.Write("notes.txt", "status: review\n")
	r.Commit("a")
	r.Checkout("main")
	r.Branch("b")
	r.Write("notes.txt", "status: final\n")
	r.Commit("b")
	claims := map[string]string{
		"agent-1/live":    `{"version":1,"branch":"agent-1/live","paths":["src/**"],"created":"2026-10-14T12:00:00Z","ttl_seconds":7200}`,
		"agent-2/newer":   `{"version":1,"branch":"agent-2/newer","paths":["lib/**"],"created":"2026-10-14T12:30:00Z","ttl_seconds":7200}`,
		"agent-3/expired": `{"version":1,"branch":"agent-3/expired","paths":["x/**"],"created":"2026-10-14T08:00:00Z","ttl_seconds":3600}`,
		"agent-4/garbage": `not json`,
	}
	for name, body := range claims {
		r.Write("claim.tmp", body)
		oid := r.Git("hash-object", "-w", "claim.tmp")
		r.Git("update-ref", "refs/mergelock/claims/"+name, oid)
	}
	r.Remove("claim.tmp")
	return r
}

func connect(t *testing.T, d Deps) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := New(d).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func deps(dir string) Deps {
	now := time.Date(2026, 10, 14, 13, 0, 0, 0, time.UTC)
	return Deps{Dir: dir, Version: "test", Rules: rules.All(), Now: func() time.Time { return now }}
}

func TestTools(t *testing.T) {
	r := repoWithConflictAndClaims(t)
	cs := connect(t, deps(r.Dir))
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	if !names["check"] || !names["claims"] || len(names) != 2 {
		t.Fatalf("tools = %v", names)
	}
}

func TestCheckTool(t *testing.T) {
	r := repoWithConflictAndClaims(t)
	cs := connect(t, deps(r.Dir))
	ctx := context.Background()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "check", Arguments: map[string]any{"branch": "a", "against": "b"}})
	if err != nil || res.IsError {
		t.Fatalf("check: %v %+v", err, res)
	}
	out := res.StructuredContent.(map[string]any)
	if out["result"] != "conflict" {
		t.Fatalf("result = %v", out["result"])
	}
	// branch defaults to the checked-out branch (b); against defaults to main.
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "check", Arguments: map[string]any{}})
	if err != nil || res.IsError || res.StructuredContent.(map[string]any)["result"] != "clean" {
		t.Fatalf("default check: %v %+v", err, res)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "check", Arguments: map[string]any{"branch": "nope"}})
	if err != nil || !res.IsError {
		t.Fatalf("unknown branch must be a tool error: %v %+v", err, res)
	}
}

func TestClaimsTool(t *testing.T) {
	r := repoWithConflictAndClaims(t)
	cs := connect(t, deps(r.Dir))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "claims", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("claims: %v %+v", err, res)
	}
	list := res.StructuredContent.(map[string]any)["claims"].([]any)
	if len(list) != 2 {
		t.Fatalf("want 2 live claims, got %v", list)
	}
	first := list[0].(map[string]any)["branch"]
	second := list[1].(map[string]any)["branch"]
	if first != "agent-2/newer" || second != "agent-1/live" {
		t.Errorf("order = %v, %v; want newest first", first, second)
	}
}
