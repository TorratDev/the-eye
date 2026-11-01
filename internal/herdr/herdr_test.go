package herdr

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Runs each hc call against the installed core on a `cat` terminal. It fails when the version
// drifts from PinnedVersion.
func TestLive(t *testing.T) {
	if _, err := exec.LookPath(Bin); err != nil {
		t.Skip("hc not installed")
	}
	v, err := Version()
	if err != nil {
		t.Fatal(err)
	}
	if v != PinnedVersion {
		t.Fatalf("hc %s installed, wrapper pinned to %s", v, PinnedVersion)
	}

	name := fmt.Sprintf("hub-test-%d", os.Getpid())
	term, err := Run(name, t.TempDir(), "echo ready; cat", map[string]string{"role": "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		exec.Command(Bin, "kill", term.Address).Run()
		exec.Command(Bin, "rm", term.Address).Run()
	})
	if term.Address == "" || term.Name != name {
		t.Fatalf("run returned %+v", term)
	}

	got, err := Get(name)
	if err != nil || got.Labels["role"] != "test" || got.State != "live" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	screenHas(t, name, "ready")

	if err := Send(name, "ping"); err != nil {
		t.Fatal(err)
	}
	screenHas(t, name, "ping") // cat echoes it once hc has typed it

	// cat never reports a status, so hc counts it as working: the wait times out
	if _, ended, err := Wait(name, time.Second); err != nil || ended {
		t.Errorf("wait on a working program: ended %v, err %v", ended, err)
	}

	if err := Keys(name, "ctrl+c"); err != nil {
		t.Fatal(err)
	}
	// ctrl+c ends cat, and Run's terminals are removed once their program exits
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		if _, err := Get(name); NotFound(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cat's terminal was not removed after ctrl+c")
		}
	}

	if _, err := Get("hub-test-no-such-terminal"); !NotFound(err) {
		t.Errorf("get of a missing terminal: %v, want not_found", err)
	}
}

func screenHas(t *testing.T, terminal, want string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		screen, err := Read(terminal, 10)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(screen, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("screen lacks %q: %q", want, screen)
		}
	}
}

// hc prints an error object with a nonzero exit when it refuses, e.g. a send to a blocked agent.
func TestDecode(t *testing.T) {
	refused := `{"error":{"hint":"read its screen first","kind":"blocked","message":"t1 is blocked (permission)"}}`
	if _, err := decode("send", []byte(refused), errors.New("exit status 3")); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Errorf("refusal: %v, want an error naming blocked", err)
	}
	if _, err := decode("get", nil, errors.New("exit status 1")); err == nil {
		t.Error("empty failure succeeded, want error")
	}
	if res, err := decode("get", []byte(`{"address":"t1"}`), nil); err != nil || !strings.Contains(string(res), "t1") {
		t.Errorf("success: %s, %v", res, err)
	}
}
