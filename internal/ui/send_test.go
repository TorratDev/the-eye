package ui

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
)

// The page's send reaches the pane through send; anything not from the hub's own page, or to a pane
// the hub has never seen, is refused before send runs.
func TestSendFromPage(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.RecordEvent(hub.Event{ReceivedAt: time.Now(), Hook: "Stop", Role: "executor", Pane: "wH:p6", Payload: "{}"}); err != nil {
		t.Fatal(err)
	}
	var got []string
	busy := false
	send := func(pane, text string) error {
		if busy {
			return errors.New("pane wH:p6 is working, not idle after 30s")
		}
		got = append(got, pane+" "+text)
		return nil
	}
	srv := httptest.NewServer(Handler(st, live.NewBroker(), nil, nil, Actions{Send: send}))
	defer srv.Close()

	post := func(pane, text string, header map[string]string) (int, string) {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/sessions/"+pane+"/send", strings.NewReader(url.Values{"text": {text}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range header {
			if k == "Host" {
				req.Host = v // the client sends req.Host, not a Host header
				continue
			}
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	own := map[string]string{"HX-Request": "true", "Origin": srv.URL}

	if code, body := post("wH:p6", "please\nrun the tests", own); code != 200 || body != "sent" {
		t.Fatalf("own page: %d %s", code, body)
	}
	if len(got) != 1 || got[0] != "wH:p6 please run the tests" {
		t.Fatalf("sent %q", got)
	}
	for name, h := range map[string]map[string]string{
		"plain form":     {"Origin": srv.URL},
		"foreign origin": {"HX-Request": "true", "Origin": "https://evil.example"},
		"rebound host":   {"HX-Request": "true", "Host": "evil.example"},
	} {
		if code, _ := post("wH:p6", "rm -rf ~", h); code != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", name, code)
		}
	}
	if code, _ := post("nobody", "hi", own); code != http.StatusNotFound {
		t.Errorf("unknown pane: %d, want 404", code)
	}
	if code, _ := post("wH:p6", "  ", own); code != http.StatusBadRequest {
		t.Errorf("empty text: %d, want 400", code)
	}
	busy = true
	if code, body := post("wH:p6", "hi", own); code != http.StatusConflict || !strings.Contains(body, "not idle") {
		t.Errorf("busy pane: %d %s", code, body)
	}
	if len(got) != 1 {
		t.Fatalf("a refused request reached send: %q", got)
	}
}
