package hub

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func openTest(t *testing.T, path string) *Store {
	t.Helper()
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestConcurrentClaimsHaveOneWinner(t *testing.T) {
	st := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := st.Claim(5, 8, fmt.Sprintf("run-%d", i))
			if err != nil {
				t.Error(err)
			}
			if ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("winners = %d, want 1", wins.Load())
	}
}

func TestClaimSurvivesRestartAndReleaseNeedsOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.Claim(1, 8, "run-a"); !ok || err != nil {
		t.Fatalf("claim = %v, %v", ok, err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st = openTest(t, path)
	claims, err := st.Claims()
	if err != nil {
		t.Fatal(err)
	}
	if claims[1] != "run-a" {
		t.Fatalf("claims after restart = %v, want 1:run-a", claims)
	}
	if err := st.Release(1, "run-b"); err != nil {
		t.Fatal(err)
	}
	if claims, _ = st.Claims(); claims[1] != "run-a" {
		t.Fatalf("non-owner released the claim: %v", claims)
	}
	if err := st.Release(1, "run-a"); err != nil {
		t.Fatal(err)
	}
	if claims, _ = st.Claims(); len(claims) != 0 {
		t.Fatalf("claims after owner release = %v, want none", claims)
	}
}

func TestReviseCounterSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := st.IncrementRevise(3); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st = openTest(t, path)
	got, err := st.IncrementRevise(3)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("revise count after restart = %d, want 3", got)
	}
}

func TestTaskAndReviseSurviveRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Task{State: "review", Batch: 2, Worktree: "C:/wt/5-state-store", Pane: "%3"}
	if err := st.SetTask(5, want); err != nil {
		t.Fatal(err)
	}
	if _, err := st.IncrementRevise(5); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st = openTest(t, path)
	got, err := st.Task(5)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("task after restart = %+v, want %+v", got, want)
	}
	var revise int
	if err := st.db.QueryRow(`SELECT revise_count FROM tasks WHERE number = 5`).Scan(&revise); err != nil {
		t.Fatal(err)
	}
	if revise != 1 {
		t.Fatalf("revise count after restart = %d, want 1", revise)
	}
}

func TestSecondOpenIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	openTest(t, path) // reopen: the lock must be taken even when the schema already exists
	st2, err := OpenStore(path)
	if err == nil {
		st2.Close()
		t.Fatal("second OpenStore succeeded while the first is open")
	}
	t.Log(err)
	if !strings.Contains(err.Error(), "locked") && !strings.Contains(err.Error(), "busy") {
		t.Fatalf("second OpenStore error = %v, want database is locked", err)
	}
}

func TestClaimSetsEpicOnReviseFirstRow(t *testing.T) {
	st := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	if _, err := st.IncrementRevise(6); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.Claim(6, 8, "run-a"); !ok || err != nil {
		t.Fatalf("claim = %v, %v", ok, err)
	}
	if _, err := st.Claim(6, 9, "run-b"); err != nil { // loser must not overwrite the epic
		t.Fatal(err)
	}
	var epic int
	if err := st.db.QueryRow(`SELECT epic FROM tasks WHERE number = 6`).Scan(&epic); err != nil {
		t.Fatal(err)
	}
	if epic != 8 {
		t.Fatalf("epic = %d, want 8", epic)
	}
}

// LastSession is the newest session id from the pane's hook events; other panes and empty ids don't count.
func TestLastSession(t *testing.T) {
	st := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	for _, e := range []Event{
		{Hook: "Stop", Role: "executor", Pane: "p6", Session: "old"},
		{Hook: "Stop", Role: "executor", Pane: "p6", Session: "new"},
		{Hook: "Stop", Role: "executor", Pane: "p6"},
		{Hook: "Stop", Role: "executor", Pane: "p7", Session: "other"},
	} {
		e.ReceivedAt = time.Now()
		if err := st.RecordEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if id, err := st.LastSession("p6"); err != nil || id != "new" {
		t.Errorf("LastSession(p6) = %q, %v", id, err)
	}
	if id, err := st.LastSession("p9"); err != nil || id != "" {
		t.Errorf("LastSession(p9) = %q, %v", id, err)
	}
}
