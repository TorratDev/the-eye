package cloud

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/TorratDev/the-eye/internal/hub"
)

// A sub-issue claimed locally is not started by a cloud worker.
func TestCloudDoesNotStartClaimedIssue(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if ok, err := st.Claim(22, 8, "executor-1"); err != nil || !ok {
		t.Fatalf("local claim: %v, %v", ok, err)
	}

	comments := []Comment{{Body: Marker + "\nrole: executor\nstarted\n\nDONE: done; verification: ok; branch: 22-x"}}
	out, err := Reconcile(st, 22, comments)
	if err != nil {
		t.Fatal(err)
	}
	if out.Applied != 0 || len(out.Refused) != 1 {
		t.Errorf("outcome = %+v, want the cloud report refused", out)
	}
	if task, _ := st.Task(22); task.State == "in review" {
		t.Error("cloud report changed the state of a locally claimed sub-issue")
	}
}

// A cloud BLOCKED report reaches the orchestrator as a routed event, whether or not the issue is claimed.
func TestCloudBlockedRoutedToOrchestrator(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	comments := []Comment{{Body: Marker + "\nrole: executor\nno dump\n\nBLOCKED: missing dump"}}
	out, err := Reconcile(st, 23, comments)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Routed) != 1 || !strings.Contains(out.Routed[0], "missing dump") {
		t.Errorf("routed = %v, want the BLOCKED reason", out.Routed)
	}
	if task, _ := st.Task(23); task.State != "blocked" {
		t.Errorf("unclaimed state = %q, want blocked", task.State)
	}
}
