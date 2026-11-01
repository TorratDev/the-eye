// Package spawner names and launches the hc terminals for one epic.
package spawner

import (
	"fmt"

	"github.com/TorratDev/the-eye/internal/herdr"
)

// Role is one role and its session slots.
type Role struct {
	Tab   string
	Panes []string
}

// Roles is the fixed set of slots from the plan: one group per role, extra sessions as more slots.
var Roles = []Role{
	{Tab: "orchestrator", Panes: []string{"orchestrator"}},
	{Tab: "investigators", Panes: []string{"investigator-a", "investigator-b", "investigator-c"}},
	{Tab: "executors", Panes: []string{"executor-1", "executor-2", "executor-3"}},
	{Tab: "reviewer", Panes: []string{"reviewer-1", "reviewer-2"}},
}

// Panes returns each slot's hc terminal name for the epic, e.g. "executor-1" -> "e147-executor-1".
// The name is the session's pane ID everywhere in the hub (store, hooks, page). Nothing is
// created: a slot's terminal starts when the slot gets work (Start).
func Panes(epic int) map[string]string {
	ids := map[string]string{}
	for _, role := range Roles {
		for _, slot := range role.Panes {
			ids[slot] = fmt.Sprintf("e%d-%s", epic, slot)
		}
	}
	return ids
}

// space is the window space the epic's terminals go in, "" when no window is open. ShowEpic sets
// it once before any session starts; the hub runs one epic per process.
var space string

// ShowEpic groups the epic's sessions in a herdr app window space named "epic <n>", made in cwd
// when it does not exist yet: terminals started from now on get the space label, and live ones
// are labeled now. With no window open it does nothing.
func ShowEpic(epic int, cwd string) error {
	id, err := herdr.AppSpace(fmt.Sprintf("epic %d", epic), cwd)
	if err != nil || id == "" {
		return err
	}
	space = id
	for _, pane := range Panes(epic) {
		if t, err := herdr.Get(pane); err == nil && t.State == "live" {
			if err := herdr.Label(pane, herdr.SpaceLabel, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// labels are a new session terminal's labels: its role, and the epic's space when a window shows it.
func labels(role string) map[string]string {
	l := map[string]string{"role": role}
	if space != "" {
		l[herdr.SpaceLabel] = space
	}
	return l
}
