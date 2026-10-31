package main

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
)

func TestWorkerResult(t *testing.T) {
	for _, c := range []struct{ role, reply, want string }{
		{"executor", "All green.\n\nDONE: added the flag; verification: go test ok; branch: 22-flag", "DONE: added the flag; verification: go test ok; branch: 22-flag"},
		{"executor", "Still working on it.", ""},
		{"reviewer", "Checked.\nREVISE: rename the flag", "REVISE: rename the flag"},
		{"investigator", "Findings:\n\n| ID | finding | tag | evidence | confidence | task | one PR |\n| P1 | uses X | [verified] | a.go:3 | high | swap X | yes |", "| ID | finding | tag | evidence | confidence | task | one PR |\n| P1 | uses X | [verified] | a.go:3 | high | swap X | yes |"},
		{"investigator", "Could not read the dump.\nBLOCKED: no RESCUE_ROM", "BLOCKED: no RESCUE_ROM"},
	} {
		if got := workerResult(c.role, c.reply); got != c.want {
			t.Errorf("%s %q: got %q, want %q", c.role, c.reply, got, c.want)
		}
	}
}

// A worker's result reaches the orchestrator tagged with its pane name and sub-issue, in order;
// a turn without a result is reported, and other roles are ignored.
func TestForwarderTagsAndOrders(t *testing.T) {
	sent := make(chan string, 8)
	f := newForwarder(map[string]string{"executor-1": "wH:p6", "reviewer-1": "wH:p9"},
		func(text string) error { sent <- text; return nil }, func(string, error) {})
	f.assign(22, "wH:p6")
	stop := func(role, pane, reply string) bool {
		payload, _ := json.Marshal(map[string]string{"last_assistant_message": reply})
		return f.onStop(hub.Event{Hook: "Stop", Role: role, Pane: pane, Payload: string(payload)})
	}
	if !stop("executor", "wH:p6", "DONE: x; verification: ok; branch: 22-x") || !stop("reviewer", "wH:p9", "PASS: fine") {
		t.Fatal("a result was not forwarded")
	}
	if stop("executor", "wH:p6", "thinking out loud") {
		t.Error("a turn without a result reported as forwarded")
	}
	if !stop("orchestrator", "wH:p2", "TASK #1: x") {
		t.Error("the orchestrator's own Stop was treated as a worker's")
	}
	f.ready([]int{23, 24})
	var got []string
	for len(got) < 3 {
		select {
		case s := <-sent:
			got = append(got, s)
		case <-time.After(2 * time.Second):
			t.Fatalf("forwarded %q", got)
		}
	}
	want := []string{"[executor-1 · #22] DONE: x; verification: ok; branch: 22-x", "[reviewer-1] PASS: fine", "Ready to plan: #23, #24."}
	if !slices.Equal(got, want) {
		t.Fatalf("forwarded %q, want %q", got, want)
	}
}

func TestNewlyReady(t *testing.T) {
	announced := map[int]bool{5: true}
	v := []hub.Verdict{{Issue: hub.Issue{Number: 5}, Ready: true}, {Issue: hub.Issue{Number: 6}, Ready: true}, {Issue: hub.Issue{Number: 7}}}
	if got := newlyReady(v, announced); !slices.Equal(got, []int{6}) {
		t.Fatalf("got %v", got)
	}
	if got := newlyReady(v, announced); len(got) != 0 {
		t.Fatalf("announced twice: %v", got)
	}
}
