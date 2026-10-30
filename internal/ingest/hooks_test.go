package ingest

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TorratDev/the-eye/internal/hub"
)

// A posted Stop event for a known pane is stored with its role and pane.
func TestStopEventStored(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := httptest.NewServer(Handler(st, nil))
	defer srv.Close()

	body := `{"hook_event_name":"Stop","session_id":"s1"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(body))
	req.Header.Set("X-Hub-Role", "executor")
	req.Header.Set("X-Hub-Pane", "executor-1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", res.StatusCode)
	}

	events, err := st.Events("executor-1")
	if err != nil || len(events) != 1 {
		t.Fatalf("Events = %v, %v; want one event", events, err)
	}
	e := events[0]
	if e.Hook != "Stop" || e.Role != "executor" || e.Pane != "executor-1" || e.Session != "s1" {
		t.Errorf("stored event = %+v", e)
	}
}

func TestRefusesBadEvents(t *testing.T) {
	cases := map[string]struct{ role, pane, body string }{
		"unknown role": {"nobody", "p1", `{"hook_event_name":"Stop"}`},
		"no pane":      {"executor", "", `{"hook_event_name":"Stop"}`},
		"unknown hook": {"executor", "p1", `{"hook_event_name":"SessionStart"}`},
		"not json":     {"executor", "p1", `Stop`},
	}
	for name, c := range cases {
		if _, err := parse(c.role, c.pane, []byte(c.body)); err == nil {
			t.Errorf("%s: accepted, want error", name)
		}
	}
}
