package session

import "testing"

func TestPermissionPromptShowsBlocked(t *testing.T) {
	s := Apply(Working, Event{Hook: "Notification", Role: "executor", Pane: "executor-1"})
	if s != Blocked {
		t.Errorf("after Notification state = %s, want blocked", s)
	}
	if s = Apply(s, Event{Hook: "PostToolUse"}); s != Working {
		t.Errorf("after the tool ran state = %s, want working", s)
	}
	if s = Apply(s, Event{Hook: "Stop"}); s != Idle {
		t.Errorf("after Stop state = %s, want idle", s)
	}
	if s = Apply(s, Event{Hook: "PostToolUseFailure"}); s != Error {
		t.Errorf("after a tool failure state = %s, want error", s)
	}
	if s = Apply(Working, Event{Hook: "SessionStart"}); s != Working {
		t.Errorf("an unmapped event changed the state to %s", s)
	}
}

func TestFromHerdr(t *testing.T) {
	if FromHerdr("done") != Idle || FromHerdr("blocked") != Blocked || FromHerdr("weird") != "" {
		t.Error("herdr status mapping is wrong")
	}
}

func TestReviseRate(t *testing.T) {
	rate, n := ReviseRate([]string{"PASS", "REVISE", "REVISE", "PASS"})
	if n != 4 || rate != 0.5 {
		t.Errorf("ReviseRate = %v, %d; want 0.5, 4", rate, n)
	}
	if rate, n := ReviseRate(nil); rate != 0 || n != 0 {
		t.Errorf("ReviseRate(nil) = %v, %d", rate, n)
	}
}
