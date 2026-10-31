package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/TorratDev/the-eye/internal/config"
	"github.com/TorratDev/the-eye/internal/herdr"
	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/prompts"
	"github.com/TorratDev/the-eye/internal/report"
	"github.com/TorratDev/the-eye/internal/spawner"
)

const planUsage = "usage: hub plan [--repo owner/name] [--config repos/<name>.md] [--cwd dir] <epic>"

// plan starts an epic with one command: sync, baseline checks, herdr layout, serve, orchestrator, hand-off.
func plan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	o := serveFlags(fs)
	cfgPath := fs.String("config", "", "per-repo config (default: repos/<name>.md)")
	cwd := fs.String("cwd", "", "the target repo's checkout, where the panes start (default: current directory)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(planUsage)
	}
	epic, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("epic must be an issue number: %w", err)
	}
	o.epic = epic
	if o.repo == "" {
		if o.repo, err = originRepo(); err != nil {
			return err
		}
	}
	owner, name, err := parseRepo(o.repo)
	if err != nil {
		return err
	}
	if *cfgPath == "" {
		*cfgPath = "repos/" + name + ".md"
	}
	if *cwd == "" {
		if *cwd, err = os.Getwd(); err != nil {
			return err
		}
	}
	var cfg config.Config
	var rendered map[string]string
	var store *hub.Store // set once serve has opened it
	var fwd *forwarder   // set once serve has opened the store
	announced := map[int]bool{}
	var handoff string
	return planSteps{
		sync: func() error {
			verdicts, err := sync(owner, name, epic)
			// the hand-off names the ready sub-issues; the watcher announces only later ones
			handoff = fmt.Sprintf("Epic #%d.", epic)
			if ready := newlyReady(verdicts, announced); len(ready) > 0 {
				handoff += " Ready to plan: " + refList(ready) + "."
			}
			return err
		},
		baseline: spawner.Baseline,
		prompt: func() (string, error) {
			if cfg, err = config.Load(*cfgPath); err != nil {
				return "", err
			}
			if rendered, err = rolePrompts(cfg); err != nil {
				return "", err
			}
			return rendered["orchestrator"], nil
		},
		layout: func() (map[string]string, error) { return spawner.Ensure(epic, *cwd) },
		serve: func(panes map[string]string) (func() error, error) {
			o.onEvent = func(st *hub.Store, broker *live.Broker) func(hub.Event) {
				store = st
				orchestrator := panes["orchestrator"]
				fwd = newForwarder(panes,
					func(text string) error { return spawner.SendWhenIdle(orchestrator, text, orchestratorWait) },
					func(text string, err error) {
						alert(st, broker, "hub", decisionsPane, fmt.Sprintf("not forwarded to the orchestrator (%v): %s", err, text))
					})
				go watchReady(st, owner, name, epic, o.poll, announced, fwd)
				d := liveDeps(st, broker, owner+"/"+name, *cwd, cfg.Header["worktree_root"], epic, panes, rendered)
				d.Assign = fwd.assign
				act := actOnStop(d, showResult(st, broker))
				repo := owner + "/" + name
				rv := newReviewer(st, panes, d.Send, fwd.assign, func(n int, err error) {
					alert(st, broker, "hub", decisionsPane, fmt.Sprintf("review of #%d: %v", n, err))
				}, func(ex spawner.ExecutorDispatch, done, verdict string) {
					url, err := openPR(ghRun, repo, ex, done, verdict)
					if err != nil {
						// the task stays approved; a later PASS for it reuses an open PR, never a second one
						alert(st, broker, "hub", decisionsPane, fmt.Sprintf("PR for #%d not opened: %v", ex.Issue, err))
						return
					}
					if err := st.SetPR(ex.Issue, url); err != nil {
						fmt.Fprintln(os.Stderr, "hub: store PR:", err)
					}
					if err := (report.GhCommenter{Repo: repo}).Comment(ex.Issue, "PR opened by the hub after review: "+url); err != nil {
						fmt.Fprintln(os.Stderr, "hub:", err)
					}
					fwd.note(fmt.Sprintf("[hub · #%d] PR opened: %s", ex.Issue, url))
				}, func(n int, verdict string) {
					fwd.note(fmt.Sprintf("[hub · #%d] REVISE %d times; latest %s. Decide the follow-up for what is left.", n, maxRevise, verdict))
				})
				return func(e hub.Event) {
					act(e)
					rv.onStop(e)
					if !fwd.onStop(e) {
						alert(st, broker, e.Role, e.Pane, "ended its turn without a result line")
					}
				}
			}
			return startServe(*o)
		},
		running: agentRunning,
		session: func(pane string) (string, error) { return store.LastSession(pane) },
		start:   spawner.StartOrchestrator,
		send:    func(pane, text string) error { return spawner.SendWhenIdle(pane, handoff, 2*time.Minute) },
	}.run(epic)
}

// watchReady announces sub-issues that become ready after the hand-off (a blocker merged).
// announced is only touched here once serve runs, after sync filled it.
func watchReady(st *hub.Store, owner, name string, epic int, every time.Duration, announced map[int]bool, fwd *forwarder) {
	token, err := hub.GhToken()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hub: ready watch disabled:", err)
		return
	}
	client := &hub.Client{URL: graphQL, Token: token}
	for range time.Tick(every) {
		issues, err := client.Epic(owner, name, epic)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hub: ready watch:", err)
			continue
		}
		claims, err := st.Claims()
		if err != nil {
			fmt.Fprintln(os.Stderr, "hub: ready watch:", err)
			continue
		}
		verdicts, _ := hub.Plan(issues, claims)
		fwd.ready(newlyReady(verdicts, announced))
	}
}

// planSteps are plan's side effects, injected so the order can be tested.
type planSteps struct {
	sync     func() error
	baseline func() error
	prompt   func() (string, error)
	layout   func() (map[string]string, error)
	serve    func(panes map[string]string) (wait func() error, err error)
	running  func(pane string) (bool, error)
	session  func(pane string) (string, error) // the pane's last hub session, to resume
	start    func(pane, prompt, resume string) error
	send     func(pane, text string) error
}

// run stops at the first failing step. The prompt renders before herdr is touched, so a bad
// config fails early, and serve is up before the orchestrator starts, so its hooks have a target.
func (s planSteps) run(epic int) error {
	if err := s.sync(); err != nil {
		return err
	}
	if err := s.baseline(); err != nil {
		return err
	}
	prompt, err := s.prompt()
	if err != nil {
		return err
	}
	panes, err := s.layout()
	if err != nil {
		return err
	}
	pane := panes["orchestrator"]
	if pane == "" {
		return errors.New("herdr layout has no orchestrator pane")
	}
	wait, err := s.serve(panes)
	if err != nil {
		return err
	}
	// a re-run must not type a second launch line into a pane that already runs the orchestrator,
	// which already has the epic from the first run
	running, err := s.running(pane)
	if err != nil {
		return err
	}
	if running {
		fmt.Printf("hub: orchestrator already running in pane %s; not starting another\n", pane)
		return wait()
	}
	// a pane the hub ran an orchestrator in before has lost it (a herdr restart, #126): resume that
	// session with the hub's flags; it already has the epic
	resume, err := s.session(pane)
	if err != nil {
		return err
	}
	if err := s.start(pane, prompt, resume); err != nil {
		return err
	}
	if resume != "" {
		fmt.Printf("hub: resumed orchestrator session %s in pane %s\n", resume, pane)
		return wait()
	}
	if err := s.send(pane, fmt.Sprintf("Epic #%d.", epic)); err != nil {
		return err
	}
	return wait()
}

// agentRunning reports whether herdr sees an agent in pane. A plain shell reports "unknown".
func agentRunning(pane string) (bool, error) {
	p, err := herdr.PaneGet(pane)
	if err != nil {
		return false, err
	}
	return p.AgentStatus != "" && p.AgentStatus != "unknown", nil
}

// rolePrompts fills each role's prompt in prompts/ from the repo config, so a bad config fails
// before any session starts.
func rolePrompts(c config.Config) (map[string]string, error) {
	out := map[string]string{}
	for _, role := range []string{"orchestrator", "investigator", "executor", "reviewer"} {
		text, err := os.ReadFile("prompts/" + role + ".md")
		if err != nil {
			return nil, err
		}
		if out[role], err = prompts.RenderRole(role, string(text), c); err != nil {
			return nil, fmt.Errorf("%s prompt: %w", role, err)
		}
	}
	return out, nil
}
