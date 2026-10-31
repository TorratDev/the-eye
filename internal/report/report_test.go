package report

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/TorratDev/the-eye/internal/hub"
)

type fakeComments struct{ bodies []string }

func (f *fakeComments) Comment(issue int, body string) error {
	f.bodies = append(f.bodies, body)
	return nil
}

// A report call creates one stored record and one issue comment.
func TestSubmitStoresAndComments(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &fakeComments{}

	if err := Submit(st, c, 22, "executor", "done", "tests pass"); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.Reports(22); n != 1 {
		t.Errorf("stored reports = %d, want 1", n)
	}
	if len(c.bodies) != 1 {
		t.Errorf("comments = %d, want 1", len(c.bodies))
	}
}

// A report that fails a check is neither stored nor commented.
func TestSubmitRefusesBadReports(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &fakeComments{}

	bad := map[string]func() error{
		"no issue":      func() error { return Submit(st, c, 0, "executor", "done", "x") },
		"unknown role":  func() error { return Submit(st, c, 22, "nobody", "done", "x") },
		"bad status":    func() error { return Submit(st, c, 22, "executor", "PASS", "x") },
		"two lines":     func() error { return Submit(st, c, 22, "executor", "done", "a\nb") },
		"empty summary": func() error { return Submit(st, c, 22, "executor", "done", "  ") },
	}
	for name, run := range bad {
		if err := run(); err == nil {
			t.Errorf("%s: accepted, want error", name)
		}
	}
	if n, _ := st.Reports(22); n != 0 || len(c.bodies) != 0 {
		t.Errorf("refused reports left %d stored and %d comments", n, len(c.bodies))
	}
}

// An executor's report ends its executing task; a reviewer's report, or a task not executing, is left alone.
func TestExecutorReportFinishesTask(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for n, state := range map[int]string{22: "executing", 23: "executing", 24: "approved"} {
		if err := st.SetTask(n, hub.Task{State: state, Pane: fmt.Sprintf("p%d", n)}); err != nil {
			t.Fatal(err)
		}
	}
	c := &fakeComments{}
	for _, r := range []struct {
		issue        int
		role, status string
	}{{22, "executor", "blocked"}, {23, "reviewer", "pass"}, {24, "executor", "done"}} {
		if err := Submit(st, c, r.issue, r.role, r.status, "x"); err != nil {
			t.Fatal(err)
		}
	}
	for n, want := range map[int]string{22: "blocked", 23: "executing", 24: "approved"} {
		if task, _ := st.Task(n); task.State != want {
			t.Errorf("#%d state %q, want %q", n, task.State, want)
		}
	}
}

// A reviewer reports a verdict, not done; other roles report done or blocked.
func TestStatusesPerRole(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &fakeComments{}
	for _, ok := range [][2]string{{"reviewer", "pass"}, {"reviewer", "REVISE"}, {"executor", "done"}, {"investigator", "blocked"}} {
		if err := Submit(st, c, 22, ok[0], ok[1], "x"); err != nil {
			t.Errorf("%s %s refused: %v", ok[0], ok[1], err)
		}
	}
	for _, bad := range [][2]string{{"reviewer", "done"}, {"executor", "pass"}} {
		if err := Submit(st, c, 22, bad[0], bad[1], "x"); err == nil {
			t.Errorf("%s %s accepted", bad[0], bad[1])
		}
	}
}
