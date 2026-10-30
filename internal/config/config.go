// Package config reads the per-repo config the hub injects into the role prompts.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Config is .orchestrator/repo.json in the target repo. See docs/repo-config.md.
type Config struct {
	RepoName      string   `json:"repo_name"`
	RepoSummary   string   `json:"repo_summary"`
	ModuleDir     string   `json:"module_dir"`
	Docs          []string `json:"docs"`
	VerifyCommand string   `json:"verify_command"`
	Rules         []string `json:"rules"`
}

// Load reads and checks the config of the repo at repoDir. An unknown field is an error,
// so a typo fails here instead of leaving a placeholder empty.
func Load(repoDir string) (Config, error) {
	path := filepath.Join(repoDir, ".orchestrator", "repo.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.check(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// check names every required field that is empty.
func (c Config) check() error {
	var missing []string
	for name, v := range map[string]string{
		"repo_name":      c.RepoName,
		"repo_summary":   c.RepoSummary,
		"module_dir":     c.ModuleDir,
		"verify_command": c.VerifyCommand,
	} {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return errors.New("missing required field(s): " + strings.Join(missing, ", "))
	}
	return nil
}
