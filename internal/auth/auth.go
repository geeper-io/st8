// Package auth provides token-based RBAC for st8d.
//
// Tokens are defined in a YAML config file. Each token carries a policy that
// restricts access to specific namespaces, branches, document key prefixes,
// and HTTP verbs (read / write / admin).
package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Verb constants used in policy definitions and enforcement.
const (
	VerbRead  = "read"
	VerbWrite = "write"
	VerbAdmin = "admin"
)

// Policy describes what a token is permitted to do.
type Policy struct {
	// Namespaces is a list of namespace glob patterns the token may access.
	// Use "*" to allow all namespaces.
	Namespaces []string `yaml:"namespaces"`

	// Branches is a list of branch glob patterns the token may access.
	// Use "*" to allow all branches.
	Branches []string `yaml:"branches"`

	// KeyPrefix restricts access to document keys that start with this prefix.
	// An empty prefix allows access to all keys.
	KeyPrefix string `yaml:"key_prefix"`

	// Verbs lists the HTTP operation classes this token may perform.
	// Valid values: "read", "write", "admin".
	Verbs []string `yaml:"verbs"`
}

// TokenEntry pairs a secret token value with a human-readable name and policy.
type TokenEntry struct {
	Token string `yaml:"token"`
	Name  string `yaml:"name"`
	Allow Policy `yaml:"allow"`
}

// Config holds all token → policy mappings loaded from a YAML file.
type Config struct {
	Tokens []TokenEntry `yaml:"tokens"`
}

// Load reads and parses an auth config from the given YAML file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read auth config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse auth config: %w", err)
	}
	return &cfg, nil
}

// AdminConfig returns a Config with a single all-access token.
// Used when only --token is provided without --auth-config.
func AdminConfig(token string) *Config {
	return &Config{
		Tokens: []TokenEntry{
			{
				Token: token,
				Name:  "admin",
				Allow: Policy{
					Namespaces: []string{"*"},
					Branches:   []string{"*"},
					KeyPrefix:  "",
					Verbs:      []string{VerbRead, VerbWrite, VerbAdmin},
				},
			},
		},
	}
}

// Principal is an authenticated caller with its resolved access policy.
type Principal struct {
	Name  string
	Allow Policy
}

// AllowsVerb reports whether the principal holds the given verb.
func (p *Principal) AllowsVerb(verb string) bool {
	for _, v := range p.Allow.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// AllowsNamespace reports whether the principal may access the given namespace.
func (p *Principal) AllowsNamespace(ns string) bool {
	return matchAny(p.Allow.Namespaces, ns)
}

// AllowsBranch reports whether the principal may access the given branch.
func (p *Principal) AllowsBranch(branch string) bool {
	return matchAny(p.Allow.Branches, branch)
}

// AllowsKey reports whether the principal may access the given document key.
// An empty KeyPrefix permits access to all keys.
func (p *Principal) AllowsKey(key string) bool {
	if p.Allow.KeyPrefix == "" {
		return true
	}
	return strings.HasPrefix(key, p.Allow.KeyPrefix)
}

// matchAny returns true when value matches at least one glob pattern in patterns.
func matchAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if pattern == "*" {
			return true
		}
		if ok, _ := filepath.Match(pattern, value); ok {
			return true
		}
	}
	return false
}

type contextKey struct{}

// ContextWithPrincipal returns a copy of ctx carrying the given principal.
func ContextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// PrincipalFromContext returns the Principal stored in ctx.
// Returns (nil, false) when no principal is present (auth not configured).
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(*Principal)
	return p, ok && p != nil
}
