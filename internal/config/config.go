// Package config reads the per-repo config the hub uses to fill the role prompts.
// The format is in prompts/README.md: header key: value lines, then ## sections.
package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// headerKeys are the keys the config may set. Only repo_name and repo_summary are injected
// into prompts; the rest are for the hub.
var headerKeys = []string{"repo_name", "repo_summary", "verify", "worktree_root", "shared_files"}

// sectionNames are the ## sections the config may have. resources is for the hub and is never injected.
var sectionNames = []string{"all", "orchestrator", "investigator", "executor", "reviewer", "resources"}

// Config is one repo's config.
type Config struct {
	Header   map[string]string
	Sections map[string]string
}

// Load reads and checks the config at path.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	c, err := Parse(string(raw))
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Parse reads the config text. It rejects unknown header keys and section names, so a typo
// fails here instead of leaving a placeholder empty.
func Parse(text string) (Config, error) {
	c := Config{Header: map[string]string{}, Sections: map[string]string{}}
	var section string // "" while still in the header
	var body []string
	flush := func() {
		if section != "" {
			c.Sections[section] = strings.TrimSpace(strings.Join(body, "\n"))
		}
		body = nil
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if name, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			name = strings.ToLower(strings.TrimSpace(name))
			if !slices.Contains(sectionNames, name) {
				return Config{}, fmt.Errorf("unknown section %q", name)
			}
			if _, dup := c.Sections[name]; dup {
				return Config{}, fmt.Errorf("section %q appears twice", name)
			}
			section = name
			c.Sections[name] = ""
			continue
		}
		if section != "" {
			body = append(body, line)
			continue
		}
		if strings.HasPrefix(line, "# ") || strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if !ok || !slices.Contains(headerKeys, key) {
			return Config{}, fmt.Errorf("unknown header line %q", line)
		}
		c.Header[key] = strings.TrimSpace(value)
	}
	flush()
	for _, key := range []string{"repo_name", "repo_summary"} {
		if c.Header[key] == "" {
			return Config{}, fmt.Errorf("missing required header %s", key)
		}
	}
	return c, nil
}

// RepoRules is the text for {{repo_rules}} in one role's prompt: the all section, then the role's own section.
func (c Config) RepoRules(role string) string {
	var parts []string
	for _, name := range []string{"all", role} {
		if s := c.Sections[name]; s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}
