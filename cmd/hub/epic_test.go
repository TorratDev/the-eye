package main

import "testing"

// A second epic in the same serve process is refused before any herdr step runs.
func TestEpicStartRefusesSecond(t *testing.T) {
	h, err := newEpicHost("o/r", "", t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	h.started = true
	if err := h.start(96, nil); err == nil {
		t.Fatal("second start succeeded, want refusal")
	}
}

// Without a repo the host refuses to start an epic and says what to pass.
func TestEpicStartNeedsRepo(t *testing.T) {
	h, err := newEpicHost("", "", t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.start(96, nil); err == nil {
		t.Fatal("start without a repo succeeded")
	}
}
