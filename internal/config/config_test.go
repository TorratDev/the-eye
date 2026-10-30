package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRepo(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".orchestrator"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".orchestrator", "repo.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad(t *testing.T) {
	good := `{"repo_name":"rust-mm","repo_summary":"DS game in Rust","module_dir":"src/","verify_command":"cargo test","docs":["docs/legal.md"],"rules":[]}`
	c, err := Load(writeRepo(t, good))
	if err != nil {
		t.Fatal(err)
	}
	if c.RepoName != "rust-mm" || c.VerifyCommand != "cargo test" || len(c.Docs) != 1 {
		t.Errorf("Load = %+v", c)
	}

	if _, err := Load(writeRepo(t, `{"repo_name":"x","repo_summary":" ","module_dir":"src/"}`)); err == nil ||
		!strings.Contains(err.Error(), "repo_summary") || !strings.Contains(err.Error(), "verify_command") {
		t.Errorf("missing fields: err = %v, want both named", err)
	}

	if _, err := Load(writeRepo(t, `{"repo_name":"x","repo_summary":"y","module_dir":"src/","verify_command":"c","verify_comand":"typo"}`)); err == nil {
		t.Error("unknown field accepted, want error")
	}

	if _, err := Load(t.TempDir()); err == nil {
		t.Error("missing file accepted, want error")
	}
}
