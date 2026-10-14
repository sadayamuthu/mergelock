package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/sadayamuthu/mergelock/internal/bench"
	"github.com/sadayamuthu/mergelock/internal/engine"
	"github.com/sadayamuthu/mergelock/internal/gitx"
	"github.com/sadayamuthu/mergelock/internal/rules"
	"github.com/sadayamuthu/mergelock/internal/verdict"
)

func TestWarmCheckUnder2s(t *testing.T) {
	if testing.Short() {
		t.Skip("performance test; run without -short")
	}
	r := bench.Generate(t, 5000, 40)
	repo, err := gitx.Open(r.Dir)
	if err != nil {
		t.Fatal(err)
	}
	opts := engine.Options{Rules: rules.All()}
	ctx := context.Background()

	start := time.Now()
	v, err := engine.Check(ctx, repo, "a", "b", opts)
	if err != nil {
		t.Fatal(err)
	}
	cold := time.Since(start)
	n := 0
	for _, f := range v.Findings {
		if f.Class == "signature_caller" && f.Confidence == verdict.ConfidenceHigh {
			n++
		}
	}
	if v.Result != verdict.Conflict || n != 40 {
		t.Fatalf("want 40 high signature_caller findings, got %d (result %s)", n, v.Result)
	}

	start = time.Now()
	if _, err := engine.Check(ctx, repo, "a", "b", opts); err != nil {
		t.Fatal(err)
	}
	warm := time.Since(start)
	t.Logf("5,000 files, 40+40 changed: cold %v, warm %v", cold, warm)
	if warm > 2*time.Second {
		t.Fatalf("warm check took %v, budget is 2s", warm)
	}
}
