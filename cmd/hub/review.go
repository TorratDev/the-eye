package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/spawner"
)

// maxRevise is how many REVISE verdicts a task gets before the orchestrator decides on a follow-up.
const maxRevise = 3

// reviewer hands DONE work to a free reviewer and REVISE work back to a free executor. Its work runs
// on one goroutine, so two results at once cannot take the same pane; work that finds no free pane
// waits for the next result to free one.
type reviewer struct {
	st         *hub.Store
	panes      []string                      // reviewer pane IDs, in order
	executors  []string                      // executor pane IDs, in order
	send       func(pane, text string) error // starts or resumes the reviewer session, then sends
	assign     func(issue int, pane string)
	failed     func(issue int, err error)
	passed     func(ex spawner.ExecutorDispatch, done, verdict string) // after a PASS, on this goroutine
	revisedOut func(issue int, verdict string)                         // after the maxRevise-th REVISE

	events    chan hub.Event
	pending   []job
	reviewing map[string]review // reviewer pane ID -> the review it was sent
}

// job is work waiting for a free pane: try returns errBusy while none is free.
type job struct {
	issue int
	try   func() error
}

type review struct {
	issue int
	done  string // the executor's DONE line
}

func newReviewer(st *hub.Store, panes map[string]string, send func(pane, text string) error, assign func(int, string), failed func(int, error),
	passed func(ex spawner.ExecutorDispatch, done, verdict string), revisedOut func(int, string)) *reviewer {
	r := &reviewer{st: st, send: send, assign: assign, failed: failed, passed: passed, revisedOut: revisedOut,
		events: make(chan hub.Event, 64), reviewing: map[string]review{}}
	for _, role := range spawner.Roles {
		for _, name := range role.Panes {
			switch {
			case panes[name] == "":
			case strings.HasPrefix(name, "reviewer-"):
				r.panes = append(r.panes, panes[name])
			case strings.HasPrefix(name, "executor-"):
				r.executors = append(r.executors, panes[name])
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
		rv := review{n, done}
		r.pending = append(r.pending, job{n, func() error { return r.start(rv) }})
	}
	if e.Role == "reviewer" {
		r.verdict(e)
	}
	var waiting []job
	for _, j := range r.pending {
		switch err := j.try(); {
		case err == errBusy:
			waiting = append(waiting, j)
		case err != nil:
			r.failed(j.issue, err)
		}
	}
	r.pending = waiting
}

var errBusy = fmt.Errorf("no free pane")

// free returns the first pane with no task executing or reviewing, or errBusy.
func (r *reviewer) free(panes []string) (string, error) {
	for _, p := range panes {
		busy, err := r.st.PaneBusy(p)
		if err != nil {
			return "", err
		}
		if !busy {
			return p, nil
		}
	}
	return "", errBusy
}

// start sends one review to a free reviewer and marks the task reviewing there.
func (r *reviewer) start(rv review) error {
	pane, err := r.free(r.panes)
	if err != nil {
		return err
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
	if change, ok := strings.CutPrefix(line, "REVISE: "); ok {
		r.reviseOrEscalate(rv.issue, line, change)
		return
	}
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

// reviseOrEscalate counts a REVISE. Under maxRevise the task goes back to a free executor with the
// same worktree and branch; at maxRevise it goes to the orchestrator instead.
func (r *reviewer) reviseOrEscalate(n int, verdict, change string) {
	count, err := r.st.IncrementRevise(n)
	if err != nil {
		r.failed(n, err)
		return
	}
	if count >= maxRevise {
		if r.revisedOut != nil {
			r.revisedOut(n, verdict)
		}
		return
	}
	r.pending = append(r.pending, job{n, func() error { return r.sendBack(n, change) }})
}

// sendBack sends a task back to a free executor with the reviewer's change, and marks it executing there.
func (r *reviewer) sendBack(n int, change string) error {
	pane, err := r.free(r.executors)
	if err != nil {
		return err
	}
	ex, err := r.task(n)
	if err != nil {
		return err
	}
	ex.Revise = change
	msg, err := ex.Message()
	if err != nil {
		return err
	}
	if err := r.send(pane, msg); err != nil {
		return err
	}
	r.assign(n, pane)
	return r.st.SetTask(n, hub.Task{State: "executing", Worktree: ex.Worktree, Pane: pane})
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
