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
		serve: func() (func() error, error) {
			return func() error { return step("wait") }, step("serve")
		},
		start: func(pane, prompt string) error { return step("start " + pane + " " + prompt) },
		send:  func(pane, text string) error { return step("send " + pane + " " + text) },
	}
}

func TestPlanOrder(t *testing.T) {
	var calls []string
	if err := fakeSteps(&calls, "").run(95); err != nil {
		t.Fatal(err)
	}
	want := []string{"sync", "baseline", "prompt", "layout", "serve", "start p1 PROMPT", "send p1 Epic #95.", "wait"}
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
