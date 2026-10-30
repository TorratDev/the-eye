package ui

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
)

// A tool error appears in the errors panel, newest first, and the progress panel counts states.
func TestErrorsAndProgress(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(st.RecordEvent(hub.Event{ReceivedAt: time.Now(), Hook: "PostToolUseFailure", Role: "executor", Pane: "executor-1", Payload: `{"error":"go test failed: exit 1"}`}))
	must(st.RecordEvent(hub.Event{ReceivedAt: time.Now(), Hook: "Notification", Role: "executor", Pane: "executor-2", Payload: `{"message":"Allow Bash?"}`}))
	must(st.SetTask(22, hub.Task{State: "claimed", Pane: "executor-1"}))
	must(st.SetTask(23, hub.Task{State: "ready"}))

	frag, err := SessionsFragment(st)
	must(err)
	if !strings.Contains(frag, "go test failed: exit 1") || !strings.Contains(frag, "Allow Bash?") {
		t.Fatalf("errors panel lacks an alert:\n%s", frag)
	}
	if strings.Index(frag, "Allow Bash?") > strings.Index(frag, "go test failed") {
		t.Errorf("alerts not newest first")
	}
	if !strings.Contains(frag, "claimed: #22") || !strings.Contains(frag, "ready: #23") {
		t.Errorf("progress panel lacks the states:\n%s", frag)
	}
}

// A tool error reaches a connected client within a second.
func TestToolErrorReachesClient(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := httptest.NewServer(Handler(st, live.NewBroker()))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := make(chan string, 256)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	post, _ := http.NewRequest(http.MethodPost, srv.URL+"/hooks",
		strings.NewReader(`{"hook_event_name":"PostToolUseFailure","error":"go test failed: exit 1"}`))
	post.Header.Set("X-Hub-Role", "executor")
	post.Header.Set("X-Hub-Pane", "executor-1")
	pr, err := http.DefaultClient.Do(post)
	if err != nil {
		t.Fatal(err)
	}
	pr.Body.Close()

	timeout := time.After(time.Second)
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatal("stream closed before the error arrived")
			}
			if strings.Contains(l, "go test failed: exit 1") {
				return
			}
		case <-timeout:
			t.Fatal("tool error did not reach the client within a second")
		}
	}
}
