package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/spawner"
)

// reviewer hands DONE work to a free reviewer. Its work runs on one goroutine, so two DONEs at once
// cannot take the same reviewer pane; a review that finds no free reviewer waits for the next verdict.
type reviewer struct {
	st     *hub.Store
	panes  []string                      // reviewer pane IDs, in order
	send   func(pane, text string) error // starts or resumes the reviewer session, then sends
	assign func(issue int, pane string)
	failed func(issue int, err error)
	passed func(ex spawner.ExecutorDispatch, done, verdict string) // after a PASS, on this goroutine

	events    chan hub.Event
	pending   []review
	reviewing map[string]review // reviewer pane ID -> the review it was sent
}

type review struct {
	issue int
	done  string // the executor's DONE line
}

func newReviewer(st *hub.Store, panes map[string]string, send func(pane, text string) error, assign func(int, string), failed func(int, error),
	passed func(ex spawner.ExecutorDispatch, done, verdict string)) *reviewer {
	r := &reviewer{st: st, send: send, assign: assign, failed: failed, passed: passed, events: make(chan hub.Event, 64), reviewing: map[string]review{}}
	for _, role := range spawner.Roles {
		for _, name := range role.Panes {
			if strings.HasPrefix(name, "reviewer-") && panes[name] != "" {
				r.panes = append(r.panes, panes[name])
			}
		}
	}
	go func() {
		for e := range r.events {
			r.handle(e)
		}
	}()
	return r
}

// onStop queues a worker's Stop event: an executor's DONE starts a review, and a reviewer's
// verdict frees a reviewer for a waiting review.
func (r *reviewer) onStop(e hub.Event) {
	if e.Hook == "Stop" && (e.Role == "executor" || e.Role == "reviewer") {
		r.events <- e
	}
}

func (r *reviewer) handle(e hub.Event) {
	if e.Role == "executor" {
		done := workerResult("executor", stopReply(e))
		if !strings.HasPrefix(done, "DONE: ") {
			return
		}
		// the hooks callback has already moved the task to in review
		n, err := r.st.PaneTask(e.Pane, "in review")
		if err != nil || n == 0 {
			r.failed(n, fmt.Errorf("DONE from %s, but no task in review there (%v)", e.Pane, err))
			return
		}
		r.pending = append(r.pending, review{n, done})
	}
	if e.Role == "reviewer" {
		r.verdict(e)
	}
	var waiting []review
	for _, rv := range r.pending {
		switch err := r.start(rv); {
		case err == errNoReviewer:
			waiting = append(waiting, rv)
		case err != nil:
			r.failed(rv.issue, err)
		}
	}
	r.pending = waiting
}

var errNoReviewer = fmt.Errorf("no free reviewer")

// start sends one review to a free reviewer and marks the task reviewing there.
func (r *reviewer) start(rv review) error {
	pane := ""
	for _, p := range r.panes {
		busy, err := r.st.PaneBusy(p)
		if err != nil {
			return err
		}
		if !busy {
			pane = p
			break
		}
	}
	if pane == "" {
		return errNoReviewer
	}
	ex, err := r.task(rv.issue)
	if err != nil {
		return err
	}
	msg, err := spawner.ReviewDispatch{Issue: ex.Issue, Objective: ex.Objective, Files: ex.Files, Verify: ex.Verify,
		Evidence: ex.Evidence, Done: rv.done, Worktree: ex.Worktree, Branch: ex.Branch}.Message()
	if err != nil {
		return err
	}
	if err := r.send(pane, msg); err != nil {
		return err
	}
	r.assign(rv.issue, pane)
	r.reviewing[pane] = rv
	return r.st.SetTask(rv.issue, hub.Task{State: "reviewing", Worktree: ex.Worktree, Pane: pane})
}

// verdict ends the review a reviewer pane was sent; a PASS goes on to passed.
func (r *reviewer) verdict(e hub.Event) {
	rv, ok := r.reviewing[e.Pane]
	if !ok {
		return
	}
	line := workerResult("reviewer", stopReply(e))
	if line == "" {
		return // no verdict yet: the reviewer is still on it
	}
	delete(r.reviewing, e.Pane)
	if !strings.HasPrefix(line, "PASS: ") || r.passed == nil {
		return
	}
	ex, err := r.task(rv.issue)
	if err != nil {
		r.failed(rv.issue, err)
		return
	}
	r.passed(ex, rv.done, line)
}

// task reads the executor dispatch kept for a sub-issue.
func (r *reviewer) task(n int) (spawner.ExecutorDispatch, error) {
	var ex spawner.ExecutorDispatch
	raw, err := r.st.Dispatch(n)
	if err != nil || raw == "" {
		return ex, fmt.Errorf("no stored task for #%d (%v)", n, err)
	}
	return ex, json.Unmarshal([]byte(raw), &ex)
}

// stopReply is the reply text a Stop event carries.
func stopReply(e hub.Event) string {
	var p struct {
		Reply string `json:"last_assistant_message"`
	}
	json.Unmarshal([]byte(e.Payload), &p)
	return p.Reply
}
