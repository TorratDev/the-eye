package main

import (
	"errors"
	"slices"
	"testing"
)

// fakeSteps records each step's name; the step named in fail returns an error.
func fakeSteps(calls *[]string, fail string) planSteps {
	step := func(name string) error {
		*calls = append(*calls, name)
		if name == fail {
			return errors.New(name + " failed")
		}
		return nil
	}
	return planSteps{
		sync:     func() error { return step("sync") },
		baseline: func() error { return step("baseline") },
		prompt:   func() (string, error) { return "PROMPT", step("prompt") },
		layout: func() (map[string]string, error) {
			return map[string]string{"orchestrator": "p1"}, step("layout")
		},
		serve: func(map[string]string) (func() error, error) {
			return func() error { return step("wait") }, step("serve")
		},
		running: func(pane string) (bool, error) { return false, step("running " + pane) },
		session: func(name string) (string, error) { return "", step("session " + name) },
		start:   func(pane, prompt, resume string) error { return step("start " + pane + " " + prompt + " " + resume) },
		send:    func(pane, text string) error { return step("send " + pane + " " + text) },
	}
}

func TestPlanOrder(t *testing.T) {
	var calls []string
	if err := fakeSteps(&calls, "").run(95); err != nil {
		t.Fatal(err)
	}
	want := []string{"sync", "baseline", "prompt", "layout", "serve", "running p1", "session orchestrator", "start p1 PROMPT ", "send p1 Epic #95.", "wait"}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

func TestPlanStopsAtFirstFailure(t *testing.T) {
	var calls []string
	if err := fakeSteps(&calls, "baseline").run(95); err == nil {
		t.Fatal("run succeeded, want the baseline error")
	}
	if want := []string{"sync", "baseline"}; !slices.Equal(calls, want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

func TestPlanNeedsOrchestratorPane(t *testing.T) {
	var calls []string
	s := fakeSteps(&calls, "")
	s.layout = func() (map[string]string, error) { return map[string]string{}, nil }
	if err := s.run(95); err == nil {
		t.Fatal("run succeeded without an orchestrator pane")
	}
}

func TestPlanSkipsLaunchWhenOrchestratorRuns(t *testing.T) {
	var calls []string
	s := fakeSteps(&calls, "")
	s.running = func(string) (bool, error) { return true, nil }
	if err := s.run(95); err != nil {
		t.Fatal(err)
	}
	if want := []string{"sync", "baseline", "prompt", "layout", "serve", "wait"}; !slices.Equal(calls, want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

// A pane whose orchestrator was lost is resumed with the hub's flags, and the epic is not sent again.
func TestPlanResumesLostOrchestrator(t *testing.T) {
	var calls []string
	s := fakeSteps(&calls, "")
	s.session = func(string) (string, error) { return "cea7c612", nil }
	if err := s.run(95); err != nil {
		t.Fatal(err)
	}
	if want := []string{"sync", "baseline", "prompt", "layout", "serve", "running p1", "start p1 PROMPT cea7c612", "wait"}; !slices.Equal(calls, want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

// A missing browser opener is skipped quietly, not an error.
func TestOpenPageSkipsWithoutBrowser(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	openPage("http://127.0.0.1:8080")
}
