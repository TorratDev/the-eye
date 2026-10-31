package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
)

func get(t *testing.T, url string) string {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %s %s", url, res.Status, b)
	}
	return string(b)
}

// The session page shows the pane's turns, the turns fragment picks up a new turn, and the
// sessions table links to the page.
func TestSessionPage(t *testing.T) {
	dir := t.TempDir()
	st, err := hub.OpenStore(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(dir, "s1.jsonl")
	write := func(lines ...string) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		for _, l := range lines {
			f.WriteString(l + "\n")
		}
	}
	write(`{"type":"user","message":{"content":"Epic #7."}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"gh issue view 7"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true,"content":"<script>x</script>"}]}}`)
	if err := st.RecordEvent(hub.Event{ReceivedAt: time.Now(), Hook: "PostToolUse", Role: "orchestrator", Pane: "wH:p2",
		Payload: `{"transcript_path":"` + path + `"}`}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(st, live.NewBroker(), nil, nil))
	defer srv.Close()

	page := get(t, srv.URL+"/sessions/wH:p2")
	for _, want := range []string{"Epic #7.", "<summary>Bash</summary>", "gh issue view 7", `class="result error"`, `hx-get="/sessions/wH:p2/turns"`, `hx-trigger="sse:state"`} {
		if !strings.Contains(page, want) {
			t.Errorf("session page lacks %q", want)
		}
	}
	if strings.Contains(page, "<script>x</script>") {
		t.Error("tool output is not escaped")
	}

	write(`{"type":"assistant","message":{"content":[{"type":"text","text":"All done."}]}}`)
	if frag := get(t, srv.URL+"/sessions/wH:p2/turns"); !strings.Contains(frag, "All done.") {
		t.Errorf("turns fragment lacks the new turn: %s", frag)
	}
	if !strings.Contains(get(t, srv.URL+"/"), `<a href="/sessions/wH:p2">`) {
		t.Error("sessions table does not link the pane")
	}
	if frag := get(t, srv.URL+"/sessions/nobody/turns"); !strings.Contains(frag, "no transcript yet") {
		t.Errorf("unknown pane: %s", frag)
	}
}
