package session

// State is what one session is doing, as the UI and the orchestrator read it.
type State string

const (
	Working State = "working"
	Idle    State = "idle"
	Blocked State = "blocked" // a permission prompt or a question is waiting for an answer
	Error   State = "error"
)

// Event is one Claude Code hook event from a session, with the role and pane from its env vars.
type Event struct {
	Hook string // Stop, Notification, PostToolUse or PostToolUseFailure
	Role string
	Pane string
}

// Apply returns the state after one hook event. Hooks are the primary signal; an event the
// model does not map leaves the state unchanged.
func Apply(cur State, e Event) State {
	switch e.Hook {
	case "Stop":
		return Idle
	case "Notification":
		return Blocked
	case "PostToolUseFailure":
		return Error
	case "PostToolUse":
		return Working
	}
	return cur
}

// FromHerdr maps herdr's agent_status to a State. It is a secondary check, used only when no
// hook event has arrived. An unknown status gives "" so the caller keeps what it has.
func FromHerdr(status string) State {
	switch status {
	case "working":
		return Working
	case "idle", "done":
		return Idle
	case "blocked":
		return Blocked
	}
	return ""
}

// ReviseRate is the share of REVISE among reviewer verdicts, and how many verdicts it counted.
func ReviseRate(verdicts []string) (float64, int) {
	if len(verdicts) == 0 {
		return 0, 0
	}
	revise := 0
	for _, v := range verdicts {
		if v == "REVISE" {
			revise++
		}
	}
	return float64(revise) / float64(len(verdicts)), len(verdicts)
}
