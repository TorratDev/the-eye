package hub

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
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
	st.Close()

	st = openTest(t, path)
	got, err := st.IncrementRevise(3)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("revise count after restart = %d, want 3", got)
	}
}
