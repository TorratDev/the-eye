package spawner

import (
	"encoding/json"
	"fmt"
	"regexp"
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

// Command returns the shell line that starts one role's session in pane, or resumes session resume
// when it is not "". The hub's role and pane ID go in as env vars for the hooks.
//
// HERDR_ENV=0 keeps herdr's SessionStart hook from recording the session. herdr resumes a recorded
// session after a restart as plain `claude --resume <id>`, without this line's hooks, env and report
// tool (#126). Unrecorded, the pane comes back as a shell, and the hub resumes the session with
// this line instead.
func Command(kind, pane, prompt, resume string) (string, error) {
	l, ok := launches[kind]
	if !ok {
		return "", fmt.Errorf("unknown role %q", kind)
	}
	cmd := fmt.Sprintf("HERDR_ENV=0 HUB_ROLE=%s HUB_PANE=%s claude --model %s --append-system-prompt %s --allowedTools %s --mcp-config %s --settings %s",
		kind, pane, l.Model, shQuote(prompt), strings.Join(l.Tools, ","), shQuote(mcpConfig), shQuote(hookSettings))
	if resume != "" {
		// the id comes from a hook body, so only a plain session id goes on the line
		if !sessionID.MatchString(resume) {
			return "", fmt.Errorf("not a session id: %q", resume)
		}
		cmd += " --resume " + resume
	}
	return cmd, nil
}

var sessionID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// mcpConfig starts the hub's report tool for every session. The server reads the role from HUB_ROLE,
// which Command sets in the same line, so the config carries no role.
const mcpConfig = `{"mcpServers":{"hub":{"command":"hub","args":["mcp"]}}}`

// hookCommand posts the hook's JSON input to the hub with the session's role and pane (docs/hooks.md).
// It never fails the hook, so a hub that is down does not block the session.
const hookCommand = `curl -s -m 5 -X POST "${HUB_URL:-http://127.0.0.1:8080}/hooks" -H "X-Hub-Role: $HUB_ROLE" -H "X-Hub-Pane: $HUB_PANE" -H "Content-Type: application/json" --data-binary @- >/dev/null || true`

// hookSettings registers hookCommand for the hooks the hub stores. --settings adds to the user's
// settings, so the RTK and herdr hooks keep running.
var hookSettings = func() string {
	entry := []map[string]any{{"hooks": []map[string]string{{"type": "command", "command": hookCommand}}}}
	hooks := map[string]any{}
	for _, name := range []string{"Stop", "Notification", "PostToolUse", "PostToolUseFailure"} {
		hooks[name] = entry
	}
	b, _ := json.Marshal(map[string]any{"hooks": hooks})
	return string(b)
}()

// Start runs the role's launch line in a new hc terminal named pane, in cwd, resuming session
// resume when it is not "". hc refuses when a live terminal already has the name.
func Start(kind, pane, cwd, prompt, resume string) error {
	cmd, err := Command(kind, pane, prompt, resume)
	if err != nil {
		return err
	}
	_, err = herdr.Run(pane, cwd, cmd, labels(kind))
	return err
}

// shQuote wraps s in single quotes for a POSIX shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
