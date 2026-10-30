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

// A hook event changes the row in the page and reaches a connected client within a second.
func TestPermissionPromptReachesClient(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := httptest.NewServer(Handler(st, live.NewBroker()))
	defer srv.Close()

	// connect first; the snapshot comes before any event
	res, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	frames := make(chan string, 8)
	go func() {
		var b strings.Builder
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			b.WriteString(sc.Text() + "\n")
			if sc.Text() == "" {
				frames <- b.String()
				b.Reset()
			}
		}
		close(frames)
	}()

	post, _ := http.NewRequest(http.MethodPost, srv.URL+"/hooks",
		strings.NewReader(`{"hook_event_name":"Notification","session_id":"s1"}`))
	post.Header.Set("X-Hub-Role", "executor")
	post.Header.Set("X-Hub-Pane", "executor-1")
	pr, err := http.DefaultClient.Do(post)
	if err != nil {
		t.Fatal(err)
	}
	pr.Body.Close()
	if pr.StatusCode != http.StatusNoContent {
		t.Fatalf("hook status = %d", pr.StatusCode)
	}

	timeout := time.After(time.Second)
	for {
		select {
		case f := <-frames:
			if strings.Contains(f, "event: state") && strings.Contains(f, "executor-1") && strings.Contains(f, "blocked") {
				// the page itself shows the same row
				page, err := http.Get(srv.URL + "/")
				if err != nil {
					t.Fatal(err)
				}
				defer page.Body.Close()
				buf := new(strings.Builder)
				sc := bufio.NewScanner(page.Body)
				for sc.Scan() {
					buf.WriteString(sc.Text())
				}
				if !strings.Contains(buf.String(), "executor-1") || !strings.Contains(buf.String(), "blocked") {
					t.Errorf("page does not show the blocked row")
				}
				return
			}
		case <-timeout:
			t.Fatal("blocked state did not reach the client within a second")
		}
	}
}
