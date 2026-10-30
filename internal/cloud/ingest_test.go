package cloud

import (
	"path/filepath"
	"testing"

	"github.com/TorratDev/the-eye/internal/hub"
)

// A cloud worker's comment changes the sub-issue state in the hub.
func TestCloudCommentChangesState(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	comments := []Comment{
		{Body: "Looking at it now."},
		{Body: Marker + "\nrole: executor\nDone.\n\nDONE: tests pass; verification: ok; branch: 22-dispatch"},
	}
	n, err := Apply(st, 22, comments)
	if err != nil || n != 1 {
		t.Fatalf("Apply = %d, %v; want 1 applied", n, err)
	}
	task, err := st.Task(22)
	if err != nil || task.State != "in review" {
		t.Errorf("state = %q, %v; want \"in review\"", task.State, err)
	}
}

// A malformed marked comment is reported, and the valid reports around it still apply.
func TestMalformedCommentReported(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	comments := []Comment{
		{Body: Marker + "\nrole: executor\nno final line"},
		{Body: Marker + "\nrole: executor\nBLOCKED: missing dump"},
	}
	n, err := Apply(st, 22, comments)
	if err == nil {
		t.Error("malformed report not reported")
	}
	if n != 1 {
		t.Errorf("applied = %d, want 1", n)
	}
	if task, _ := st.Task(22); task.State != "blocked" {
		t.Errorf("state = %q, want blocked", task.State)
	}
}
