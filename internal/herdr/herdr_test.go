package herdr

import (
	"errors"
	"os/exec"
	"testing"
)

// Runs the read-only calls against the installed herdr. It fails when the version drifts from PinnedVersion.
func TestPinnedVersion(t *testing.T) {
	if _, err := exec.LookPath(Bin); err != nil {
		t.Skip("herdr not installed")
	}
	v, err := Version()
	if err != nil {
		t.Fatal(err)
	}
	if v != PinnedVersion {
		t.Fatalf("herdr %s installed, wrapper pinned to %s", v, PinnedVersion)
	}
	if _, err := WorkspaceList(); err != nil {
		t.Fatal(err)
	}
	if _, err := AgentRead("", 1); err == nil {
		t.Fatal("AgentRead with an empty target succeeded, want error")
	}
}

// pane run prints nothing when it succeeds; a failed command with no output is still an error.
func TestDecodeEmptyOutput(t *testing.T) {
	if _, err := decode("pane", nil, nil); err != nil {
		t.Errorf("empty success: %v", err)
	}
	if _, err := decode("pane", nil, errors.New("exit status 1")); err == nil {
		t.Error("empty failure succeeded, want error")
	}
	if _, err := decode("pane", []byte(`{"error":{"code":"x","message":"y"}}`), nil); err == nil {
		t.Error("error envelope succeeded, want error")
	}
}
