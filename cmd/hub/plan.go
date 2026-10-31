package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/TorratDev/the-eye/internal/config"
	"github.com/TorratDev/the-eye/internal/prompts"
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
	return planSteps{
		sync:     func() error { return sync(owner, name, epic) },
		baseline: spawner.Baseline,
		prompt:   func() (string, error) { return orchestratorPrompt(*cfgPath) },
		layout:   func() (map[string]string, error) { return spawner.Ensure(epic, *cwd) },
		serve:    func() (func() error, error) { return startServe(*o) },
		start:    spawner.StartOrchestrator,
		send:     func(pane, text string) error { return spawner.SendWhenIdle(pane, text, 2*time.Minute) },
	}.run(epic)
}

// planSteps are plan's side effects, injected so the order can be tested.
type planSteps struct {
	sync     func() error
	baseline func() error
	prompt   func() (string, error)
	layout   func() (map[string]string, error)
	serve    func() (wait func() error, err error)
	start    func(pane, prompt string) error
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
	wait, err := s.serve()
	if err != nil {
		return err
	}
	if err := s.start(pane, prompt); err != nil {
		return err
	}
	if err := s.send(pane, fmt.Sprintf("Epic #%d.", epic)); err != nil {
		return err
	}
	return wait()
}

// orchestratorPrompt fills prompts/orchestrator.md from the repo config at cfgPath.
func orchestratorPrompt(cfgPath string) (string, error) {
	c, err := config.Load(cfgPath)
	if err != nil {
		return "", err
	}
	text, err := os.ReadFile("prompts/orchestrator.md")
	if err != nil {
		return "", err
	}
	return prompts.RenderRole("orchestrator", string(text), c)
}
