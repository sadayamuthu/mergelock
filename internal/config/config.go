// Package config loads .mergelock.yml. Only `target` and `rules` affect v0.1;
// the other keys from the product spec are accepted and ignored.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"
)

const (
	FileName      = ".mergelock.yml"
	DefaultTarget = "main"
)

var knownKeys = map[string]bool{
	"target": true, "test": true, "block_on": true, "min_confidence": true,
	"strict_claims": true, "lock_timeout_seconds": true, "rules": true,
}

// RuleIDs are the rule ids `rules:` may configure: every class in the
// product spec, including those v0.1 does not implement yet (configuring
// them is accepted and has no effect).
var RuleIDs = []string{
	"textual", "signature_caller", "modify_delete", "add_add", "duplicate_definition",
	"interface_drift", "registry_hotspot", "test_contract", "claim_drift",
}

type Config struct {
	Target string
	rules  map[string]yaml.Node
}

func Default() *Config {
	return &Config{Target: DefaultTarget, rules: map[string]yaml.Node{}}
}

// Load reads dir/.mergelock.yml. A missing file means defaults.
func Load(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Config, error) {
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", FileName, err)
	}
	c := Default()
	for k, v := range raw {
		if !knownKeys[k] {
			return nil, fmt.Errorf("invalid %s: unknown key %q", FileName, k)
		}
		switch k {
		case "target":
			var s string
			if err := v.Decode(&s); err != nil || s == "" {
				return nil, fmt.Errorf("invalid %s: target must be a branch name", FileName)
			}
			c.Target = s
		case "rules":
			if v.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("invalid %s: rules must be a mapping of rule id to settings", FileName)
			}
			if err := v.Decode(&c.rules); err != nil {
				return nil, fmt.Errorf("invalid %s: rules: %w", FileName, err)
			}
			if err := c.validateRules(); err != nil {
				return nil, err
			}
		}
	}
	return c, nil
}

// RuleEnabled is false only when the rule is set to the scalar `off` or `false`.
func (c *Config) RuleEnabled(id string) bool {
	n, ok := c.rules[id]
	if !ok || n.Kind != yaml.ScalarNode {
		return true
	}
	return n.Value != "off" && n.Value != "false"
}

// RuleFloat reads rules.<id>.<key> as a number, or returns def.
func (c *Config) RuleFloat(id, key string, def float64) float64 {
	n, ok := c.rules[id]
	if !ok || n.Kind != yaml.MappingNode {
		return def
	}
	var m map[string]yaml.Node
	if err := n.Decode(&m); err != nil {
		return def
	}
	v, ok := m[key]
	if !ok {
		return def
	}
	var f float64
	if err := v.Decode(&f); err != nil {
		return def
	}
	return f
}

func (c *Config) validateRules() error {
	ids := make([]string, 0, len(c.rules))
	for id := range c.rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !slices.Contains(RuleIDs, id) {
			return fmt.Errorf("invalid %s: unknown rule %q under rules", FileName, id)
		}
	}
	n, ok := c.rules["duplicate_definition"]
	if !ok || n.Kind != yaml.MappingNode {
		return nil
	}
	var m map[string]yaml.Node
	if err := n.Decode(&m); err != nil {
		return fmt.Errorf("invalid %s: rules.duplicate_definition: %w", FileName, err)
	}
	if v, ok := m["similarity"]; ok {
		var f float64
		if err := v.Decode(&f); err != nil || f < 0 || f > 1 {
			return fmt.Errorf("invalid %s: rules.duplicate_definition.similarity must be a number between 0 and 1, got %q", FileName, v.Value)
		}
	}
	return nil
}
