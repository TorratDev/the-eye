package cloud

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/TorratDev/the-eye/internal/hub"
)

func openStore(t *testing.T) *hub.Store {
	t.Helper()
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

const doneComment = Marker + "\nrole: executor\nDone.\n\nDONE: tests pass; verification: ok; branch: 22-x"

func fixed(comments map[int][]Comment) Fetcher {
	return func(_ string, n int) ([]Comment, error) { return comments[n], nil }
}

// A marked cloud DONE comment changes the sub-issue state on the next tick, and is applied only once.
func TestTickAppliesOnce(t *testing.T) {
	st := openStore(t)
	comments := map[int][]Comment{22: {{Body: doneComment, CreatedAt: "2026-10-30T10:00:00Z"}}}
	applied := 0
	p := &Poller{
		Store:     st,
		Repo:      "o/r",
		Fetch:     fixed(comments),
		Issues:    func() ([]int, error) { return []int{22}, nil },
		OnOutcome: func(_ int, out Outcome) { applied += out.Applied },
	}
	for i := 0; i < 2; i++ {
		if err := p.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	if applied != 1 {
		t.Errorf("applied %d times across two ticks, want 1", applied)
	}
	if task, _ := st.Task(22); task.State != "in review" {
		t.Errorf("state = %q, want in review", task.State)
	}
}

// After a restart, a comment already applied is not replayed over newer local state.
func TestRestartDoesNotReplay(t *testing.T) {
	st := openStore(t)
	comments := map[int][]Comment{22: {{Body: doneComment, CreatedAt: "2026-10-30T10:00:00Z"}}}
	newPoller := func() *Poller {
		return &Poller{Store: st, Repo: "o/r", Fetch: fixed(comments),
			Issues: func() ([]int, error) { return []int{22}, nil }}
	}
	if err := newPoller().Tick(); err != nil {
		t.Fatal(err)
	}
	if err := st.SetState(22, "merged"); err != nil { // local state moves on
		t.Fatal(err)
	}
	if err := newPoller().Tick(); err != nil { // the hub restarts and polls again
		t.Fatal(err)
	}
	if task, _ := st.Task(22); task.State != "merged" {
		t.Errorf("state = %q after restart, want merged (the old report was replayed)", task.State)
	}
}

// A cloud BLOCKED comment is reported to OnOutcome as routed for the orchestrator.
func TestBlockedIsRouted(t *testing.T) {
	st := openStore(t)
	comments := map[int][]Comment{23: {{Body: Marker + "\nrole: executor\nstuck\n\nBLOCKED: missing dump", CreatedAt: "2026-10-30T10:00:00Z"}}}
	var routed []string
	p := &Poller{Store: st, Repo: "o/r", Fetch: fixed(comments),
		Issues:    func() ([]int, error) { return []int{23}, nil },
		OnOutcome: func(_ int, out Outcome) { routed = append(routed, out.Routed...) }}
	if err := p.Tick(); err != nil {
		t.Fatal(err)
	}
	if len(routed) != 1 {
		t.Errorf("routed = %v, want the BLOCKED report", routed)
	}
}

// A failed sub-issue is reported and the others still run.
func TestOneFailureDoesNotStopOthers(t *testing.T) {
	st := openStore(t)
	fetch := func(_ string, n int) ([]Comment, error) {
		if n == 22 {
			return nil, errors.New("gh: rate limited")
		}
		return []Comment{{Body: doneComment, CreatedAt: "2026-10-30T10:00:00Z"}}, nil
	}
	var failed []int
	p := &Poller{Store: st, Repo: "o/r", Fetch: fetch,
		Issues:  func() ([]int, error) { return []int{22, 23}, nil },
		OnError: func(n int, _ error) { failed = append(failed, n) }}
	if err := p.Tick(); err != nil {
		t.Fatal(err)
	}
	if len(failed) != 1 || failed[0] != 22 {
		t.Errorf("failed = %v, want [22]", failed)
	}
	if task, _ := st.Task(23); task.State != "in review" {
		t.Errorf("sub-issue 23 state = %q, want in review", task.State)
	}
}
