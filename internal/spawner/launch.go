package spawner

import (
	"fmt"
	"strings"

	"github.com/TorratDev/the-eye/internal/herdr"
)

// Launch is the model and tool allowance for one role. Investigators get no Edit or Write,
// so they cannot change code even when a prompt asks them to.
type Launch struct {
	Model string
	Tools []string
}

var launches = map[string]Launch{
	"orchestrator": {Model: "claude-haiku-5-5", Tools: []string{"Read", "Grep", "Glob", "Bash"}},
	"investigator": {Model: "claude-haiku-5-5", Tools: []string{"Read", "Grep", "Glob", "Bash"}},
	"executor":     {Model: "claude-haiku-5-5", Tools: []string{"Read", "Grep", "Glob", "Edit", "Write", "Bash"}},
	"reviewer":     {Model: "claude-sonnet-5-5", Tools: []string{"Read", "Grep", "Glob", "Bash"}},
}

// Command returns the shell line that starts one role's session in pane. The hub's role and
// pane ID go in as env vars for the hooks.
func Command(kind, pane, prompt string) (string, error) {
	l, ok := launches[kind]
	if !ok {
		return "", fmt.Errorf("unknown role %q", kind)
	}
	return fmt.Sprintf("HUB_ROLE=%s HUB_PANE=%s claude --model %s --append-system-prompt %s --allowedTools %s --mcp-config %s",
		kind, pane, l.Model, shQuote(prompt), strings.Join(l.Tools, ","), shQuote(mcpConfig)), nil
}

// mcpConfig starts the hub's report tool for every session. The server reads the role from HUB_ROLE,
// which Command sets in the same line, so the config carries no role.
const mcpConfig = `{"mcpServers":{"hub":{"command":"hub","args":["mcp"]}}}`

// Start types the role's launch line into an existing pane at its shell prompt.
func Start(kind, pane, prompt string) error {
	cmd, err := Command(kind, pane, prompt)
	if err != nil {
		return err
	}
	return herdr.PaneRun(pane, cmd)
}

// shQuote wraps s in single quotes for a POSIX shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
