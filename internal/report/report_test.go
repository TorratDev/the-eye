package report

import (
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
