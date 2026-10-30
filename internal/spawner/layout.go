// Package spawner lays out and launches the herdr workspace for one epic.
package spawner

import (
	"fmt"

	"github.com/TorratDev/the-eye/internal/herdr"
)

// Role is one tab in the epic workspace and the named panes it holds.
type Role struct {
	Tab   string
	Panes []string
}

// Roles is the fixed layout from the plan: one tab per role, extra sessions as panes.
var Roles = []Role{
	{Tab: "orchestrator", Panes: []string{"orchestrator"}},
	{Tab: "investigators", Panes: []string{"investigator-a", "investigator-b", "investigator-c"}},
	{Tab: "executors", Panes: []string{"executor-1", "executor-2", "executor-3"}},
	{Tab: "reviewer", Panes: []string{"reviewer-1", "reviewer-2"}},
}

// step is one change to a tab: name a free pane, or split a new one off anchor and name it.
type step struct {
	split  bool
	anchor string // pane to split, or the free pane to name
	name   string
}

// planPanes returns the steps that give every wanted name a pane. Labelled panes that are
// not wanted are left alone. Free panes are named before any new split is made.
func planPanes(wanted []string, panes []herdr.Pane) ([]step, error) {
	if len(panes) == 0 {
		return nil, fmt.Errorf("tab has no panes")
	}
	have := map[string]bool{}
	var free []string
	for _, p := range panes {
		if p.Label == "" {
			free = append(free, p.ID)
		} else {
			have[p.Label] = true
		}
	}
	var steps []step
	for _, name := range wanted {
		switch {
		case have[name]:
		case len(free) > 0:
			steps = append(steps, step{anchor: free[0], name: name})
			free = free[1:]
		default:
			steps = append(steps, step{split: true, anchor: panes[0].ID, name: name})
		}
	}
	return steps, nil
}

// Ensure makes the epic's workspace match Roles and returns pane name -> pane ID.
// It reuses the workspace labelled for the epic and creates only what is missing, so a
// second run changes nothing.
func Ensure(epic int, cwd string) (map[string]string, error) {
	label := fmt.Sprintf("epic-%d", epic)
	ws, err := findWorkspace(label, cwd)
	if err != nil {
		return nil, err
	}
	tabs, err := herdr.TabList(ws.ID)
	if err != nil {
		return nil, err
	}
	tabByLabel := map[string]herdr.Tab{}
	for _, t := range tabs {
		tabByLabel[t.Label] = t
	}
	ids := map[string]string{}
	for _, role := range Roles {
		tab, ok := tabByLabel[role.Tab]
		if !ok {
			if tab, err = herdr.TabCreate(ws.ID, cwd, role.Tab); err != nil {
				return nil, err
			}
		}
		panes, err := panesOf(ws.ID, tab.ID)
		if err != nil {
			return nil, err
		}
		steps, err := planPanes(role.Panes, panes)
		if err != nil {
			return nil, fmt.Errorf("tab %s: %w", role.Tab, err)
		}
		for _, s := range steps {
			pane := s.anchor
			if s.split {
				p, err := herdr.PaneSplit(s.anchor, "down", cwd)
				if err != nil {
					return nil, err
				}
				pane = p.ID
			}
			if err := herdr.PaneRename(pane, s.name); err != nil {
				return nil, err
			}
		}
		panes, err = panesOf(ws.ID, tab.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range panes {
			if p.Label != "" {
				ids[p.Label] = p.ID
			}
		}
	}
	return ids, nil
}

// findWorkspace returns the epic's workspace, creating it on the first run.
func findWorkspace(label, cwd string) (herdr.Workspace, error) {
	all, err := herdr.WorkspaceList()
	if err != nil {
		return herdr.Workspace{}, err
	}
	for _, w := range all {
		if w.Label == label {
			return w, nil
		}
	}
	return herdr.WorkspaceCreate(cwd, label)
}

// panesOf returns the panes of one tab.
func panesOf(workspace, tab string) ([]herdr.Pane, error) {
	all, err := herdr.PaneList(workspace)
	if err != nil {
		return nil, err
	}
	var out []herdr.Pane
	for _, p := range all {
		if p.TabID == tab {
			out = append(out, p)
		}
	}
	return out, nil
}
