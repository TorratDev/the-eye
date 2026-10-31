package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/TorratDev/the-eye/internal/config"
	"github.com/TorratDev/the-eye/internal/herdr"
	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/prompts"
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
	h, err := newEpicHost(o.repo, *cfgPath, *cwd, o.poll)
	if err != nil {
		return err
	}
	o.onEvent = h.wire
	o.startEpic = h.startFromPage
	return h.start(epic, func() (func() error, error) {
		wait, err := startServe(*o)
		if err == nil {
			openPage("http://" + o.addr)
		}
		return wait, err
	})
}

// openPage opens url in the default browser. It is a convenience: a machine with no browser
// is not an error, so a failure is skipped quietly.
func openPage(url string) {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	cmd := exec.Command(name, url)
	if err := cmd.Start(); err == nil {
		go cmd.Wait() // reap the opener so it does not linger as a zombie
	}
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
	session  func(name string) (string, error) // the named pane's last hub session, to resume
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
	resume, err := s.session("orchestrator")
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
