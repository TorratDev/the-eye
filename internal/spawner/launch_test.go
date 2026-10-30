package spawner

import (
	"slices"
	"strings"
	"testing"
)

func TestCommandRoles(t *testing.T) {
	inv, err := Command("investigator", "p3", "it's read only")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"HUB_ROLE=investigator", "HUB_PANE=p3", "--model claude-haiku-5-5", `--allowedTools Read,Grep,Glob,Bash`, `'it'\''s read only'`} {
		if !strings.Contains(inv, want) {
			t.Errorf("investigator command missing %q: %s", want, inv)
		}
	}
	if strings.Contains(inv, "Edit") || strings.Contains(inv, "Write") {
		t.Errorf("investigator allowed an edit tool: %s", inv)
	}

	rev, _ := Command("reviewer", "p9", "x")
	if !strings.Contains(rev, "--model claude-sonnet-5-5") {
		t.Errorf("reviewer not on sonnet: %s", rev)
	}
	if _, err := Command("nobody", "p1", ""); err == nil {
		t.Error("unknown role succeeded, want error")
	}
}

func TestExecutorCanEdit(t *testing.T) {
	if !slices.Contains(launches["executor"].Tools, "Edit") {
		t.Error("executor cannot edit")
	}
}

// Every role's session runs the hub's report tool through --mcp-config.
func TestEveryRoleHasReportTool(t *testing.T) {
	for kind := range launches {
		cmd, err := Command(kind, "p1", "x")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--mcp-config") || !strings.Contains(cmd, `"command":"hub"`) {
			t.Errorf("%s launch line lacks the hub report server: %s", kind, cmd)
		}
	}
}
