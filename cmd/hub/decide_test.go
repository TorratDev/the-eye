package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/decision"
	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/spawner"
	"github.com/TorratDev/the-eye/internal/ui"
)

// postStop sends a Stop hook for role through the hub's real handler.
func postStop(t *testing.T, url, role, reply string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "last_assistant_message": reply})
	req, _ := http.NewRequest(http.MethodPost, url+"/hooks", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Role", role)
	req.Header.Set("X-Hub-Pane", "p-"+role)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /hooks: %s", res.Status)
	}
}

// The orchestrator's Stop event with a TASK line dispatches that task to an executor; another
// role's Stop dispatches nothing.
func TestStopDispatchesTask(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	type sent struct{ pane, text string }
	sends := make(chan sent, 4)
	results := make(chan decision.Result, 4)
	d := decision.Deps{
		Claim:            func(int, string) (bool, error) { return true, nil },
		Worktree:         func(n int, slug string) (string, error) { return "/wt/22-" + slug, nil },
		FreeExecutorPane: func() (string, error) { return "p-exec", nil },
		Send:             func(pane, text string) error { sends <- sent{pane, text}; return nil },
		SetTask:          func(*spawner.ExecutorDispatch, string) error { return nil },
	}
	srv := httptest.NewServer(ui.Handler(st, live.NewBroker(), nil, actOnStop(d, func(r decision.Result, err error) {
		if err != nil {
			t.Error(err)
		}
		results <- r
	}), ui.Actions{}))
	defer srv.Close()

	postStop(t, srv.URL, "executor", "TASK #23: slug=no | objective=o | files=a.go | verify=v | evidence=e")
	postStop(t, srv.URL, "orchestrator", "Plan:\n\n```\nTASK #22: slug=dispatch | objective=build | files=a.go | verify=go test | evidence=output\n```")

	select {
	case r := <-results:
		if len(r.Dispatched) != 1 || len(r.Failed) != 0 {
			t.Fatalf("result %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no decision acted on")
	}
	s := <-sends
	if s.pane != "p-exec" || !strings.Contains(s.text, "#22") || !strings.Contains(s.text, "/wt/22-dispatch") {
		t.Fatalf("sent %+v", s)
	}
	if len(sends) != 0 || len(results) != 0 {
		t.Fatal("the executor's Stop was acted on")
	}
}

// With all three executors busy there is no free pane; an executor's DONE final line on its Stop
// hook frees its pane, and a reply without a final line frees nothing.
func TestDoneFreesExecutorPane(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	panes := map[string]string{"executor-1": "p-executor", "executor-2": "p-e2", "executor-3": "p-e3"}
	for n, name := range map[int]string{21: "executor-1", 22: "executor-2", 23: "executor-3"} {
		if err := st.SetTask(n, hub.Task{State: "executing", Pane: panes[name]}); err != nil {
			t.Fatal(err)
		}
	}
	free := liveDeps(st, live.NewBroker(), "o/r", "/repo", "", 1, panes, nil).FreeExecutorPane
	if p, err := free(); err != nil || p != "" {
		t.Fatalf("all busy: free pane %q, %v", p, err)
	}
	srv := httptest.NewServer(ui.Handler(st, live.NewBroker(), nil, nil, ui.Actions{}))
	defer srv.Close()

	postStop(t, srv.URL, "executor", "Working on it, tests next.")
	if p, _ := free(); p != "" {
		t.Fatalf("a reply without a final line freed %q", p)
	}
	postStop(t, srv.URL, "executor", "All checks pass.\n\nDONE: opened PR #99 for #21")
	if p, err := free(); err != nil || p != "p-executor" {
		t.Fatalf("after DONE: free pane %q, %v", p, err)
	}
	if task, _ := st.Task(21); task.State != "in review" {
		t.Fatalf("task #21 state %q, want in review", task.State)
	}
}
