package spawner

import (
	"strings"
	"testing"
)

func TestExecutorMessage(t *testing.T) {
	d := ExecutorDispatch{
		Issue: 22, Worktree: "/wt/22", Branch: "22-dispatch", Objective: "build the messages",
		Files: []string{"internal/spawner/message.go"}, Verify: "go test ./...", Evidence: "test output",
	}
	msg, err := d.Message()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"role: executor", "issue: #22", "branch: 22-dispatch", "verify: go test ./..."} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}

	d.Branch = "23-other"
	if _, err := d.Message(); err == nil {
		t.Error("branch for another issue accepted")
	}
}

func TestInvestigatorLetter(t *testing.T) {
	if _, err := (InvestigatorDispatch{Letter: "D", Issue: 22, Points: []string{"P1"}}).Message(); err == nil {
		t.Error("letter D accepted")
	}
	msg, err := (InvestigatorDispatch{Letter: "B", Issue: 22, Points: []string{"P1", "P2"}}).Message()
	if err != nil || !strings.Contains(msg, "points: P1, P2") || strings.Contains(msg, "resources") {
		t.Errorf("msg = %q, %v", msg, err)
	}
}
