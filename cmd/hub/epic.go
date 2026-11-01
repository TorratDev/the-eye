package main

import (
	"errors"
	"fmt"
	"os"
	gosync "sync"
	"time"

	"github.com/TorratDev/the-eye/internal/config"
	"github.com/TorratDev/the-eye/internal/herdr"
	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/report"
	"github.com/TorratDev/the-eye/internal/spawner"
)

// epicHost runs one epic inside a serve process. The store, broker and hook wiring are set up once
// by startServe; start names the epic's hc terminals and starts the orchestrator, from the CLI (hub plan) or from
// the page (the epic form). One epic runs per process.
type epicHost struct {
	owner, name string
	cfgPath     string
	cwd         string
	poll        time.Duration

	st     *hub.Store // set by wire, when serve opens the store
	broker *live.Broker
	cfg    config.Config
	prompt map[string]string

	mu      gosync.Mutex
	started bool
	run     *epicRun // nil until the epic's panes are attached
}

// epicRun is the epic's hook handling, built once its panes exist.
type epicRun struct {
	act func(hub.Event)
	rv  *reviewer
	fwd *forwarder
}

// newEpicHost reads the repo and defaults the config path and the checkout. An empty repo leaves
// the host unable to start an epic, which is reported when one is asked for.
func newEpicHost(repo, cfgPath, cwd string, poll time.Duration) (*epicHost, error) {
	h := &epicHost{cfgPath: cfgPath, cwd: cwd, poll: poll}
	if repo != "" {
		owner, name, err := parseRepo(repo)
		if err != nil {
			return nil, err
		}
		h.owner, h.name = owner, name
		if h.cfgPath == "" {
			h.cfgPath = "repos/" + name + ".md"
		}
	}
	if h.cwd == "" {
		var err error
		if h.cwd, err = os.Getwd(); err != nil {
			return nil, err
		}
	}
	return h, nil
}

// wire is the serveOpts.onEvent hook: it gives the host the store and broker, and returns the
// handler for hook events.
func (h *epicHost) wire(st *hub.Store, broker *live.Broker) func(hub.Event) {
	h.st, h.broker = st, broker
	return h.onEvent
}

// onEvent acts on one hook event for the running epic; before an epic has panes it does nothing.
func (h *epicHost) onEvent(e hub.Event) {
	h.mu.Lock()
	run := h.run
	h.mu.Unlock()
	if run == nil {
		return
	}
	run.act(e)
	run.rv.onStop(e)
	if !run.fwd.onStop(e) {
		alert(h.st, h.broker, e.Role, e.Pane, "ended its turn without a result line")
	}
}

// startFromPage starts an epic from the watcher page, in the serve process that is already up.
func (h *epicHost) startFromPage(epic int) error {
	return h.start(epic, nil)
}

// start runs the epic's steps. listen starts the server when the CLI has not started it yet; nil
// means serve is already up.
func (h *epicHost) start(epic int, listen func() (func() error, error)) error {
	if h.owner == "" {
		return errors.New("no GitHub repo: pass --repo, or run from a checkout with an origin remote")
	}
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return errors.New("an epic is already running here; restart hub serve to run another")
	}
	h.started = true
	h.mu.Unlock()

	var handoff string
	announced := map[int]bool{}
	err := planSteps{
		sync: func() error {
			verdicts, err := sync(h.owner, h.name, epic)
			// the hand-off names the ready sub-issues; the watcher announces only later ones
			handoff = fmt.Sprintf("Epic #%d.", epic)
			if ready := newlyReady(verdicts, announced); len(ready) > 0 {
				handoff += " Ready to plan: " + refList(ready) + "."
			}
			return err
		},
		baseline: spawner.Baseline,
		prompt: func() (string, error) {
			cfg, err := config.Load(h.cfgPath)
			if err != nil {
				return "", err
			}
			rendered, err := rolePrompts(cfg)
			if err != nil {
				return "", err
			}
			h.cfg, h.prompt = cfg, rendered
			return rendered["orchestrator"], nil
		},
		layout: func() (map[string]string, error) {
			// the window is only a view: a failure is shown, the run goes on
			if err := spawner.ShowEpic(epic, h.cwd); err != nil {
				fmt.Fprintln(os.Stderr, "hub: app window:", err)
			}
			return spawner.Panes(epic), nil
		},
		serve: func(panes map[string]string) (func() error, error) {
			wait := func() error { return nil }
			if listen != nil {
				var err error
				if wait, err = listen(); err != nil {
					return nil, err
				}
			}
			return wait, h.attach(epic, panes, announced)
		},
		running: agentRunning,
		session: func(name string) (string, error) { return h.st.LastSession(spawner.Panes(epic)[name]) },
		start: func(pane, prompt, resume string) error {
			return spawner.StartOrchestrator(pane, h.cwd, prompt, resume)
		},
		send: func(pane, text string) error { return spawner.SendWhenIdle(pane, handoff, 2*time.Minute) },
	}.run(epic)
	if err != nil && !h.attached() {
		// nothing was attached, so a later start may try again
		h.mu.Lock()
		h.started = false
		h.mu.Unlock()
	}
	return err
}

// attach builds the epic's hook handling for its panes.
func (h *epicHost) attach(epic int, panes map[string]string, announced map[int]bool) error {
	orchestrator := panes["orchestrator"]
	fwd := newForwarder(panes,
		func(text string) error { return herdr.Send(orchestrator, text) },
		func(text string, err error) {
			alert(h.st, h.broker, "hub", decisionsPane, fmt.Sprintf("not forwarded to the orchestrator (%v): %s", err, text))
		})
	go watchReady(h.st, h.owner, h.name, epic, h.poll, announced, fwd)
	d := liveDeps(h.st, h.broker, h.owner+"/"+h.name, h.cwd, h.cfg.Header["worktree_root"], epic, panes, h.prompt)
	d.Assign = fwd.assign
	repo := h.owner + "/" + h.name
	rv := newReviewer(h.st, panes, d.Send, fwd.assign, func(n int, err error) {
		alert(h.st, h.broker, "hub", decisionsPane, fmt.Sprintf("review of #%d: %v", n, err))
	}, func(ex spawner.ExecutorDispatch, done, verdict string) {
		url, err := openPR(gitRun, ghRun, repo, ex, done, verdict)
		if err != nil {
			// the task stays approved; a later PASS for it reuses an open PR, never a second one
			alert(h.st, h.broker, "hub", decisionsPane, fmt.Sprintf("PR for #%d not opened: %v", ex.Issue, err))
			return
		}
		if err := h.st.SetPR(ex.Issue, url); err != nil {
			fmt.Fprintln(os.Stderr, "hub: store PR:", err)
		}
		if err := (report.GhCommenter{Repo: repo}).Comment(ex.Issue, "PR opened by the hub after review: "+url); err != nil {
			fmt.Fprintln(os.Stderr, "hub:", err)
		}
		fwd.note(fmt.Sprintf("[hub · #%d] PR opened: %s", ex.Issue, url))
	}, func(n int, verdict string) {
		fwd.note(fmt.Sprintf("[hub · #%d] REVISE %d times; latest %s. Decide the follow-up for what is left.", n, maxRevise, verdict))
	})
	h.mu.Lock()
	h.run = &epicRun{act: actOnStop(d, showResult(h.st, h.broker)), rv: rv, fwd: fwd}
	h.mu.Unlock()
	return nil
}

func (h *epicHost) attached() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.run != nil
}
