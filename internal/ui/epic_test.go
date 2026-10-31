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

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
)

// The epic form starts an epic only from the hub's own page, with an issue number.
func TestStartEpicFromPage(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var started []int
	fail := false
	start := func(epic int) error {
		if fail {
			return errors.New("epic already running")
		}
		started = append(started, epic)
		return nil
	}
	srv := httptest.NewServer(Handler(st, live.NewBroker(), nil, nil, Actions{StartEpic: start}))
	defer srv.Close()

	post := func(epic string, header map[string]string) (int, string) {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/epics", strings.NewReader(url.Values{"epic": {epic}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range header {
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

	if code, body := post("95", own); code != 200 || body != "epic #95 started" {
		t.Fatalf("own page: %d %q", code, body)
	}
	if len(started) != 1 || started[0] != 95 {
		t.Fatalf("started %v, want [95]", started)
	}
	if code, _ := post("95", map[string]string{"Origin": srv.URL}); code != http.StatusForbidden {
		t.Errorf("plain form: %d, want 403", code)
	}
	if code, _ := post("0", own); code != http.StatusBadRequest {
		t.Errorf("epic 0: %d, want 400", code)
	}
	if code, _ := post("x", own); code != http.StatusBadRequest {
		t.Errorf("epic x: %d, want 400", code)
	}
	fail = true
	if code, body := post("96", own); code != http.StatusConflict || !strings.Contains(body, "already running") {
		t.Errorf("refused start: %d %q", code, body)
	}
}

// Without a start action the route is not configured, and the page still renders the form.
func TestStartEpicNotConfigured(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := httptest.NewServer(Handler(st, live.NewBroker(), nil, nil, Actions{}))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/epics", strings.NewReader("epic=95"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", srv.URL)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", res.StatusCode)
	}
}
