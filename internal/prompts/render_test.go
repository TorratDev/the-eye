package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TorratDev/the-eye/internal/config"
)

const repoText = `# rust-mm

repo_name: rust-mm
repo_summary: DS game reimplementation in Rust

## all
No ROM dumps in the repo.

## executor
Edit only the named files.
`

// Every real role prompt renders with no placeholder left in it.
func TestRenderRolesNoPlaceholderLeft(t *testing.T) {
	c, err := config.Parse(repoText)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"orchestrator", "investigator", "executor", "reviewer"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "prompts", role+".md"))
		if err != nil {
			t.Fatal(err)
		}
		out, err := RenderRole(role, string(raw), c)
		if err != nil {
			t.Fatalf("%s: %v", role, err)
		}
		if strings.Contains(out, "{{") {
			t.Errorf("%s: placeholder left after render", role)
		}
		if !strings.Contains(out, "No ROM dumps in the repo.") {
			t.Errorf("%s: all section not injected", role)
		}
	}
}

func TestRenderFailsOnUnfilled(t *testing.T) {
	_, err := Render("a {{repo_name}} b {{repo_name}} c {{ extra }}", map[string]string{"repo_name": "x"})
	if err == nil || !strings.Contains(err.Error(), "extra") {
		t.Errorf("err = %v, want unfilled extra", err)
	}
	out, err := Render("{{repo_name}} {{ repo_name }}", map[string]string{"repo_name": "x"})
	if err != nil || out != "x x" {
		t.Errorf("Render = %q, %v", out, err)
	}
}

// The real rust-mm config fills every role prompt with no placeholder left.
func TestRustMMConfigRenders(t *testing.T) {
	c, err := config.Load(filepath.Join("..", "..", "repos", "rust-mm.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"orchestrator", "investigator", "executor", "reviewer"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "prompts", role+".md"))
		if err != nil {
			t.Fatal(err)
		}
		out, err := RenderRole(role, string(raw), c)
		if err != nil {
			t.Fatalf("%s: %v", role, err)
		}
		if strings.Contains(out, "{{") || !strings.Contains(out, "rust-mm") {
			t.Errorf("%s: not rendered from rust-mm config", role)
		}
	}
}
