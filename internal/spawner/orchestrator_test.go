package spawner

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseLogin(t *testing.T) {
	cases := map[string]string{
		"not logged in": `{"loggedIn":false,"authMethod":"claude.ai"}`,
		"api key":       `{"loggedIn":true,"authMethod":"api_key"}`,
	}
	for name, raw := range cases {
		if err := parseLogin([]byte(raw)); err == nil {
			t.Errorf("%s: accepted, want error", name)
		}
	}
	if err := parseLogin([]byte(`{"loggedIn":true,"authMethod":"claude.ai"}`)); err != nil {
		t.Errorf("claude.ai login refused: %v", err)
	}
}

// Without the login the launcher reports the missing login and types nothing into the pane.
func TestNoLoginStartsNothing(t *testing.T) {
	typed := false
	err := startOrchestrator(
		func() error { return errors.New("not logged in: run `claude auth login`") },
		false, "orchestrator", "prompt", "",
		func(pane, cmd string) error { typed = true; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "claude auth login") {
		t.Errorf("err = %v, want the missing login named", err)
	}
	if typed {
		t.Error("launch line was typed without a login")
	}
}

func TestChannelFlagOnlyWhenAvailable(t *testing.T) {
	var got string
	run := func(pane, cmd string) error { got = cmd; return nil }
	ok := func() error { return nil }

	if err := startOrchestrator(ok, true, "orchestrator", "p", "", run); err != nil || !strings.HasSuffix(got, ChannelFlag) || !strings.HasPrefix(got, "HUB_CHANNEL=1 ") {
		t.Errorf("with channels: cmd = %q, err = %v", got, err)
	}
	if err := startOrchestrator(ok, false, "orchestrator", "p", "", run); err != nil || strings.Contains(got, ChannelFlag) || strings.Contains(got, "HUB_CHANNEL") {
		t.Errorf("without channels: cmd = %q, err = %v", got, err)
	}
}

func TestHasChannels(t *testing.T) {
	for in, want := range map[string]bool{
		"2.1.316 (Claude Code)\n": true,
		"2.1.80 (Claude Code)":    true,
		"2.1.79 (Claude Code)":    false,
		"2.0.999":                 false,
		"3.0.0":                   true,
		"garbage":                 false,
	} {
		if got := hasChannels(in); got != want {
			t.Errorf("hasChannels(%q) = %v, want %v", in, got, want)
		}
	}
}

// The prompt is answered only once it shows, and a refused Enter (pane not yet blocked) is retried.
func TestAcceptChannels(t *testing.T) {
	reads, enters := 0, 0
	read := func() (string, error) {
		reads++
		if reads < 3 {
			return "starting", nil
		}
		return "WARNING: Loading development channels\n❯ 1. I am using this for local development", nil
	}
	enter := func() error {
		enters++
		if enters == 1 {
			return errors.New("not blocked yet")
		}
		return nil
	}
	if err := acceptChannels(read, enter, time.Second, time.Millisecond); err != nil || enters != 2 {
		t.Errorf("err = %v, enters = %d", err, enters)
	}
	never := func() (string, error) { return "starting", nil }
	if err := acceptChannels(never, func() error { t.Error("enter without the prompt"); return nil }, 5*time.Millisecond, time.Millisecond); err == nil {
		t.Error("no prompt: want a timeout error")
	}
}
