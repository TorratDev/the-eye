// Package herdr is the only place the hub talks to herdr. herdr's API is not stable,
// so every call goes through here and the version is pinned.
package herdr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PinnedVersion is the herdr release this wrapper is tested against. Move it only with the test.
const PinnedVersion = "0.9.5"

// Bin is the herdr executable.
var Bin = "herdr"

// Workspace, Tab and Pane hold the fields the spawner reads from herdr's JSON.
type Workspace struct {
	ID    string `json:"workspace_id"`
	Label string `json:"label"`
}

type Tab struct {
	ID          string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
}

type Pane struct {
	ID          string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	Label       string `json:"label"`
	AgentStatus string `json:"agent_status"` // idle, working, blocked, done or unknown
}

// Version returns the installed herdr version, e.g. "0.9.5".
func Version() (string, error) {
	out, err := exec.Command(Bin, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "herdr "), nil
}

// call runs one herdr command and returns the result object of its JSON envelope.
func call(args ...string) (json.RawMessage, error) {
	out, err := exec.Command(Bin, args...).Output()
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if jerr := json.Unmarshal(out, &env); jerr != nil {
		return nil, fmt.Errorf("herdr %s: unreadable output (%v): %s", args[0], err, strings.TrimSpace(string(out)))
	}
	if env.Error != nil {
		return nil, fmt.Errorf("herdr %s: %s: %s", args[0], env.Error.Code, env.Error.Message)
	}
	if err != nil {
		return nil, fmt.Errorf("herdr %s: %w", args[0], err)
	}
	return env.Result, nil
}

// WorkspaceList returns every workspace.
func WorkspaceList() ([]Workspace, error) {
	res, err := call("workspace", "list")
	if err != nil {
		return nil, err
	}
	var v struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	return v.Workspaces, json.Unmarshal(res, &v)
}

// WorkspaceCreate opens a workspace for one epic.
func WorkspaceCreate(cwd, label string) (Workspace, error) {
	res, err := call("workspace", "create", "--cwd", cwd, "--label", label, "--no-focus")
	if err != nil {
		return Workspace{}, err
	}
	var v struct {
		Workspace Workspace `json:"workspace"`
	}
	return v.Workspace, json.Unmarshal(res, &v)
}

// TabCreate opens one role's tab inside a workspace.
func TabCreate(workspace, cwd, label string) (Tab, error) {
	res, err := call("tab", "create", "--workspace", workspace, "--cwd", cwd, "--label", label, "--no-focus")
	if err != nil {
		return Tab{}, err
	}
	var v struct {
		Tab Tab `json:"tab"`
	}
	return v.Tab, json.Unmarshal(res, &v)
}

// PaneSplit splits pane to the right or down and returns the new pane.
func PaneSplit(pane, direction, cwd string) (Pane, error) {
	res, err := call("pane", "split", pane, "--direction", direction, "--cwd", cwd)
	if err != nil {
		return Pane{}, err
	}
	var v struct {
		Pane Pane `json:"pane"`
	}
	return v.Pane, json.Unmarshal(res, &v)
}

// PaneRun types a command into a pane's shell.
func PaneRun(pane string, command ...string) error {
	_, err := call(append([]string{"pane", "run", pane}, command...)...)
	return err
}

// AgentRead returns the recent terminal text of an agent pane.
func AgentRead(target string, lines int) (string, error) {
	out, err := exec.Command(Bin, "agent", "read", target, "--source", "recent", "--lines", fmt.Sprint(lines)).Output()
	return string(out), err
}

// TabList returns the tabs of one workspace.
func TabList(workspace string) ([]Tab, error) {
	res, err := call("tab", "list", "--workspace", workspace)
	if err != nil {
		return nil, err
	}
	var v struct {
		Tabs []Tab `json:"tabs"`
	}
	return v.Tabs, json.Unmarshal(res, &v)
}

// PaneList returns the panes of one workspace.
func PaneList(workspace string) ([]Pane, error) {
	res, err := call("pane", "list", "--workspace", workspace)
	if err != nil {
		return nil, err
	}
	var v struct {
		Panes []Pane `json:"panes"`
	}
	return v.Panes, json.Unmarshal(res, &v)
}

// PaneRename sets a pane's label.
func PaneRename(pane, label string) error {
	_, err := call("pane", "rename", pane, label)
	return err
}

// PaneGet returns one pane, including its agent status.
func PaneGet(pane string) (Pane, error) {
	res, err := call("pane", "get", pane)
	if err != nil {
		return Pane{}, err
	}
	var v struct {
		Pane Pane `json:"pane"`
	}
	return v.Pane, json.Unmarshal(res, &v)
}
