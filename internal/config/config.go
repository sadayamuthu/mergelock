// Package config loads .mergelock.yml. Only `target` and `rules` affect v0.1;
// the other keys from the product spec are accepted and ignored.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
