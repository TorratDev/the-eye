package spawner

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// Every role's session posts its hooks to the hub, and the hooks run without the hub env unset.
func TestEveryRolePostsHooks(t *testing.T) {
	var s struct {
		Hooks map[string][]struct {
			Hooks []struct{ Type, Command string }
		}
	}
	if err := json.Unmarshal([]byte(hookSettings), &s); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Stop", "Notification", "PostToolUse", "PostToolUseFailure"} {
		if len(s.Hooks[name]) != 1 || s.Hooks[name][0].Hooks[0].Command != hookCommand {
			t.Errorf("%s hook not registered: %s", name, hookSettings)
		}
	}
	for kind := range launches {
		cmd, _ := Command(kind, "p1", "x")
		if !strings.Contains(cmd, "--settings "+shQuote(hookSettings)) {
			t.Errorf("%s launch line lacks the hook settings: %s", kind, cmd)
		}
	}
}

// The hook command posts the event body with the role and pane headers the ingest handler needs.
func TestHookCommandPosts(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("no curl")
	}
	type post struct{ path, role, pane, body string }
	got := make(chan post, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- post{r.URL.Path, r.Header.Get("X-Hub-Role"), r.Header.Get("X-Hub-Pane"), string(body)}
	}))
	defer srv.Close()
	c := exec.Command("sh", "-c", hookCommand)
	c.Env = append(os.Environ(), "HUB_URL="+srv.URL, "HUB_ROLE=executor", "HUB_PANE=p7")
	c.Stdin = strings.NewReader(`{"hook_event_name":"Stop"}`)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("hook command: %v %s", err, out)
	}
	if p, want := <-got, (post{"/hooks", "executor", "p7", `{"hook_event_name":"Stop"}`}); p != want {
		t.Fatalf("hook posted %+v, want %+v", p, want)
	}
}
