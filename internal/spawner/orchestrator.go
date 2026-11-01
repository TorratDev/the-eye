package spawner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/TorratDev/the-eye/internal/herdr"
)

// ChannelFlag enables development channels from the hub MCP server (mcpConfig names it "hub"). The
// relay (internal/relay) needs it; claude before channelsSince does not have it, in which case the
// orchestrator gets events through its pane instead.
const ChannelFlag = "--dangerously-load-development-channels server:hub"

// channelsSince is the first claude version with channels.
var channelsSince = [3]int{2, 1, 80}

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

// ChannelFlagAvailable reports whether this claude build accepts ChannelFlag. `claude --help` does
// not list the flag, so the version decides.
func ChannelFlagAvailable() bool {
	out, err := exec.Command("claude", "--version").Output()
	return err == nil && hasChannels(string(out))
}

// hasChannels reads `claude --version` output such as "2.1.316 (Claude Code)".
func hasChannels(version string) bool {
	var v [3]int
	if n, _ := fmt.Sscanf(version, "%d.%d.%d", &v[0], &v[1], &v[2]); n != 3 {
		return false
	}
	for i := range v {
		if v[i] != channelsSince[i] {
			return v[i] > channelsSince[i]
		}
	}
	return true
}

// startOrchestrator checks the login, then runs the orchestrator's launch line for pane.
// Nothing runs when the check fails. Channels are added only when the build has the flag.
func startOrchestrator(check func() error, channels bool, pane, prompt, resume string, run func(pane, cmd string) error) error {
	if err := check(); err != nil {
		return err
	}
	cmd, err := Command("orchestrator", pane, prompt, resume)
	if err != nil {
		return err
	}
	if channels {
		// HUB_CHANNEL tells `hub mcp` to relay the hub's messages into this session
		cmd = "HUB_CHANNEL=1 " + cmd + " " + ChannelFlag
	}
	return run(pane, cmd)
}

// channelDialog is text from the warning claude shows on every launch with ChannelFlag.
const channelDialog = "local development"

// acceptChannels waits for the development-channels warning in the pane and takes its default
// ("I am using this for local development"). enter is retried until it succeeds.
func acceptChannels(read func() (string, error), enter func() error, timeout, poll time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		screen, err := read()
		if err != nil {
			return err
		}
		if strings.Contains(screen, channelDialog) && enter() == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no development-channels prompt to accept after %s", timeout)
		}
		time.Sleep(poll)
	}
}

// StartOrchestrator launches the orchestrator in a new hc terminal named pane, in cwd, after the
// login check, resuming session resume when it is not "".
func StartOrchestrator(pane, cwd, prompt, resume string) error {
	channels := ChannelFlagAvailable()
	return startOrchestrator(CheckLogin, channels, pane, prompt, resume, func(pane, cmd string) error {
		if _, err := herdr.Run(pane, cwd, cmd, map[string]string{"role": "orchestrator"}); err != nil || !channels {
			return err
		}
		// the screen text is the guard: claude may not report blocked for this startup dialog
		return acceptChannels(
			func() (string, error) { return herdr.Read(pane, 40) },
			func() error { return herdr.Keys(pane, "enter") },
			time.Minute, time.Second)
	})
}
