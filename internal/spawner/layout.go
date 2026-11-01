// Package spawner names and launches the hc terminals for one epic.
package spawner

import "fmt"

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
