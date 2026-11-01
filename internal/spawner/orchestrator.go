package spawner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/TorratDev/the-eye/internal/herdr"
)

// parseLogin reads `claude auth status --json`. The hub runs its sessions on a claude.ai login,
// not an API key.
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
		return fmt.Errorf("logged in with %q, not claude.ai: run `claude auth login`", s.AuthMethod)
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

// startOrchestrator checks the login, then runs the orchestrator's launch line for pane.
// Nothing runs when the check fails.
func startOrchestrator(check func() error, pane, prompt, resume string, run func(pane, cmd string) error) error {
	if err := check(); err != nil {
		return err
	}
	cmd, err := Command("orchestrator", pane, prompt, resume)
	if err != nil {
		return err
	}
	return run(pane, cmd)
}

// StartOrchestrator launches the orchestrator in a new hc terminal named pane, in cwd, after the
// login check, resuming session resume when it is not "".
func StartOrchestrator(pane, cwd, prompt, resume string) error {
	return startOrchestrator(CheckLogin, pane, prompt, resume, func(pane, cmd string) error {
		_, err := herdr.Run(pane, cwd, cmd, labels("orchestrator"))
		return err
	})
}
