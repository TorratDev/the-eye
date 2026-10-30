package spawner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/TorratDev/the-eye/internal/herdr"
)

// ChannelFlag enables development channels. The relay (internal/relay) needs it; this build of
// claude may not have it, in which case the orchestrator gets events through its pane instead.
const ChannelFlag = "--dangerously-load-development-channels"

// parseLogin reads `claude auth status --json`. Channels need a claude.ai login; an API key login
// does not work for them.
func parseLogin(raw []byte) error {
	var s struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("claude auth status: unreadable output: %w", err)
	}
	if !s.LoggedIn {
		return errors.New("not logged in: run `claude auth login`")
	}
	if s.AuthMethod != "claude.ai" {
		return fmt.Errorf("logged in with %q, but channels need a claude.ai login: run `claude auth login`", s.AuthMethod)
	}
	return nil
}

// CheckLogin fails with a clear message unless claude is logged in with claude.ai.
func CheckLogin() error {
	out, err := exec.Command("claude", "auth", "status", "--json").Output()
	if err != nil {
		return fmt.Errorf("claude auth status: %w", err)
	}
	return parseLogin(out)
}

// ChannelFlagAvailable reports whether this claude build accepts ChannelFlag.
func ChannelFlagAvailable() bool {
	out, err := exec.Command("claude", "--help").CombinedOutput()
	return err == nil && strings.Contains(string(out), ChannelFlag)
}

// startOrchestrator checks the login, then types the orchestrator's launch line into pane.
// Nothing is typed when the check fails. Channels are added only when the build has the flag.
func startOrchestrator(check func() error, channels bool, pane, prompt string, run func(pane, cmd string) error) error {
	if err := check(); err != nil {
		return err
	}
	cmd, err := Command("orchestrator", pane, prompt)
	if err != nil {
		return err
	}
	if channels {
		cmd += " " + ChannelFlag
	}
	return run(pane, cmd)
}

// StartOrchestrator launches the orchestrator in pane after the login check.
func StartOrchestrator(pane, prompt string) error {
	return startOrchestrator(CheckLogin, ChannelFlagAvailable(), pane, prompt, func(pane, cmd string) error {
		return herdr.PaneRun(pane, cmd)
	})
}
