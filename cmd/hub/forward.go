package main

import (
	"encoding/json"
	"fmt"
	"strings"
	gosync "sync"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/session"
)

// orchestratorWait is how long a forwarded message waits for the orchestrator to finish a turn.
const orchestratorWait = 10 * time.Minute

// forwarder sends the orchestrator what its prompt waits for: worker results and ready sub-issues.
// Messages go out one at a time, in order, each once the orchestrator is idle.
type forwarder struct {
	names  map[string]string // pane ID -> pane name
	send   func(text string) error
	failed func(text string, err error)

	mu       gosync.Mutex
	assigned map[string]int // pane ID -> sub-issue it was last sent
	queue    chan string
}

func newForwarder(panes map[string]string, send func(text string) error, failed func(string, error)) *forwarder {
	f := &forwarder{names: map[string]string{}, send: send, failed: failed, assigned: map[string]int{}, queue: make(chan string, 64)}
	for name, id := range panes {
		f.names[id] = name
	}
	go func() {
		for text := range f.queue {
			if err := f.send(text); err != nil {
				f.failed(text, err)
			}
		}
	}()
	return f
}

// assign records which sub-issue a pane was sent, so its result can be tagged with it.
func (f *forwarder) assign(issue int, pane string) {
	f.mu.Lock()
	f.assigned[pane] = issue
	f.mu.Unlock()
}

// onStop forwards a worker's result from its Stop event. It reports false when the turn ended
// without a result the contract allows, so the caller can show that instead.
func (f *forwarder) onStop(e hub.Event) bool {
	if e.Hook != "Stop" || (e.Role != "investigator" && e.Role != "executor" && e.Role != "reviewer") {
		return true
	}
	var p struct {
		Reply string `json:"last_assistant_message"`
	}
	json.Unmarshal([]byte(e.Payload), &p)
	result := workerResult(e.Role, p.Reply)
	if result == "" {
		return false
	}
	f.mu.Lock()
	issue := f.assigned[e.Pane]
	f.mu.Unlock()
	tag := f.names[e.Pane]
	if tag == "" {
		tag = e.Pane
	}
	if issue > 0 {
		tag += fmt.Sprintf(" · #%d", issue)
	}
	f.queue <- fmt.Sprintf("[%s] %s", tag, result)
	return true
}

// ready tells the orchestrator about sub-issues ready to plan.
func (f *forwarder) ready(issues []int) {
	if len(issues) > 0 {
		f.queue <- "Ready to plan: " + refList(issues) + "."
	}
}

// workerResult is what the orchestrator needs from a worker's reply: an executor's or reviewer's
// final line, or an investigator's findings rows (or its BLOCKED line). "" when there is none.
func workerResult(role, reply string) string {
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if f, err := session.ParseFinal(role, last); err == nil && f.Kind != "FINDING" {
		return last
	}
	if role != "investigator" {
		return ""
	}
	var rows []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); strings.Count(l, "|") >= 6 {
			rows = append(rows, l)
		}
	}
	return strings.Join(rows, "\n")
}

// newlyReady returns the open, ready sub-issues not announced before, and marks them announced.
func newlyReady(verdicts []hub.Verdict, announced map[int]bool) []int {
	var out []int
	for _, v := range verdicts {
		if v.Ready && !announced[v.Issue.Number] {
			announced[v.Issue.Number] = true
			out = append(out, v.Issue.Number)
		}
	}
	return out
}
