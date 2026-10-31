package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/spawner"
	"github.com/TorratDev/the-eye/internal/ui"
)

func postStopFrom(t *testing.T, url, role, pane, reply string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"hook_event_name": "Stop", "session_id": "s-" + pane, "last_assistant_message": reply})
	req, _ := http.NewRequest(http.MethodPost, url+"/hooks", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Role", role)
	req.Header.Set("X-Hub-Pane", pane)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
}

// An executor's DONE sends a review of that task to a free reviewer. With both reviewers busy the
// next review waits, and goes out when a verdict frees a reviewer.
func TestDoneDispatchesReview(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	panes := map[string]string{"executor-1": "e1", "executor-2": "e2", "executor-3": "e3", "reviewer-1": "r1", "reviewer-2": "r2"}
	for i, n := range []int{21, 22, 23} {
		ex := spawner.ExecutorDispatch{Issue: n, Worktree: "/wt/" + string(rune('a'+i)), Branch: "2" + string(rune('1'+i)) + "-x",
			Objective: "do it", Files: []string{"a.go"}, Verify: "go test", Evidence: "e"}
		raw, _ := json.Marshal(ex)
		st.SetDispatch(n, string(raw))
		st.SetTask(n, hub.Task{State: "executing", Worktree: ex.Worktree, Pane: "e" + string(rune('1'+i))})
	}
	type sent struct{ pane, text string }
	sends := make(chan sent, 8)
	passed := make(chan string, 4)
	rv := newReviewer(st, panes, func(pane, text string) error { sends <- sent{pane, text}; return nil },
		func(int, string) {}, func(n int, err error) { t.Errorf("#%d: %v", n, err) },
		func(ex spawner.ExecutorDispatch, done, verdict string) {
			passed <- fmt.Sprintf("#%d %s | %s", ex.Issue, done, verdict)
		})
	srv := httptest.NewServer(ui.Handler(st, live.NewBroker(), nil, rv.onStop, ui.Actions{}))
	defer srv.Close()
	next := func() sent {
		t.Helper()
		select {
		case s := <-sends:
			return s
		case <-time.After(3 * time.Second):
			t.Fatal("no review sent")
		}
		return sent{}
	}

	postStopFrom(t, srv.URL, "executor", "e1", "Done.\nDONE: did it; verification: go test ok; branch: 21-x")
	s := next()
	for _, want := range []string{"role: reviewer", "issue: #21", "executor: DONE: did it; verification: go test ok; branch: 21-x", "worktree: /wt/a", "branch: 21-x"} {
		if !strings.Contains(s.text, want) {
			t.Errorf("review message lacks %q:\n%s", want, s.text)
		}
	}
	if task, _ := st.Task(21); s.pane != "r1" || task.State != "reviewing" || task.Pane != "r1" {
		t.Fatalf("review went to %s, task %+v", s.pane, task)
	}
	postStopFrom(t, srv.URL, "executor", "e2", "DONE: b; verification: ok; branch: 22-x")
	if s := next(); s.pane != "r2" || !strings.Contains(s.text, "issue: #22") {
		t.Fatalf("second review: %+v", s)
	}
	postStopFrom(t, srv.URL, "executor", "e3", "DONE: c; verification: ok; branch: 23-x")
	select {
	case s := <-sends:
		t.Fatalf("a review was sent with both reviewers busy: %+v", s)
	case <-time.After(300 * time.Millisecond):
	}
	postStopFrom(t, srv.URL, "reviewer", "r1", "Looks right.\nPASS: matches the objective")
	if s := next(); s.pane != "r1" || !strings.Contains(s.text, "issue: #23") {
		t.Fatalf("waiting review: %+v", s)
	}
	if task, _ := st.Task(21); task.State != "approved" {
		t.Fatalf("#21 after PASS: %+v", task)
	}
	if p := <-passed; p != "#21 DONE: did it; verification: go test ok; branch: 21-x | PASS: matches the objective" {
		t.Fatalf("passed %q", p)
	}
	postStopFrom(t, srv.URL, "reviewer", "r2", "REVISE: rename the flag")
	select {
	case p := <-passed:
		t.Fatalf("a REVISE opened a PR: %q", p)
	case <-time.After(300 * time.Millisecond):
	}
}
