package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sadayamuthu/mergelock/internal/config"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	c, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.Target != "main" || !c.RuleEnabled("signature_caller") {
		t.Fatalf("defaults wrong: %+v", c)
	}
	if got := c.RuleFloat("duplicate_definition", "similarity", 0.85); got != 0.85 {
		t.Fatalf("RuleFloat default = %v", got)
	}
}

func TestParseProductSpecExample(t *testing.T) {
	src := `target: develop
test: npm test
block_on: [error]
min_confidence: high
strict_claims: false
lock_timeout_seconds: 1800
rules:
  test_contract: off
  duplicate_definition:
    similarity: 0.9
  registry_hotspot:
    paths: ["src/routes/index.ts", "src/di/container.ts"]
`
	c, err := config.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if c.Target != "develop" {
		t.Errorf("Target = %q", c.Target)
	}
	if c.RuleEnabled("test_contract") {
		t.Error("test_contract should be off")
	}
	if !c.RuleEnabled("registry_hotspot") || !c.RuleEnabled("textual") {
		t.Error("mapping-configured and unlisted rules should be enabled")
	}
	if got := c.RuleFloat("duplicate_definition", "similarity", 0.85); got != 0.9 {
		t.Errorf("similarity = %v", got)
	}
	if got := c.RuleFloat("registry_hotspot", "similarity", 0.5); got != 0.5 {
		t.Errorf("missing key should give default, got %v", got)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"unknown key":  "targett: main\n",
		"bad yaml":     "target: [unclosed\n",
		"empty target": "target: \"\"\n",
		"rules list":   "rules: [a, b]\n",
	}
	for name, src := range cases {
		_, err := config.Parse([]byte(src))
		if err == nil || !strings.Contains(err.Error(), ".mergelock.yml") {
			t.Errorf("%s: err = %v, want error naming .mergelock.yml", name, err)
		}
	}
	if _, err := config.Parse([]byte("targett: main\n")); !strings.Contains(err.Error(), `"targett"`) {
		t.Errorf("unknown key error should name the key: %v", err)
	}
}

func TestLoadReadsFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, config.FileName), []byte("target: trunk\n"), 0o644)
	c, err := config.Load(dir)
	if err != nil || c.Target != "trunk" {
		t.Fatalf("Load = %+v, %v", c, err)
	}
	empty := t.TempDir()
	os.WriteFile(filepath.Join(empty, config.FileName), nil, 0o644)
	if c, err := config.Load(empty); err != nil || c.Target != "main" {
		t.Fatalf("empty file should give defaults: %+v, %v", c, err)
	}
}
