package live

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A connected client receives a state change within a second, after the snapshot.
func TestClientReceivesChangeWithinASecond(t *testing.T) {
	b := NewBroker()
	srv := httptest.NewServer(b.Handler(func() string { return "event: snapshot\ndata: {}\n\n" }))
	defer srv.Close()

	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	// the snapshot arrives first, then the client is subscribed
	waitFor(t, lines, "event: snapshot")
	deadline := time.Now().Add(time.Second)
	for {
		b.Publish("state", `{"pane":"executor-1","state":"blocked"}`)
		if line, ok := tryRead(lines, 50*time.Millisecond); ok && line == "event: state" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("state change not received within a second")
		}
	}
}

func waitFor(t *testing.T, lines chan string, want string) {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("stream closed before %q", want)
			}
			if strings.HasPrefix(l, want) {
				return
			}
		case <-timeout:
			t.Fatalf("no %q within a second", want)
		}
	}
}

func tryRead(lines chan string, d time.Duration) (string, bool) {
	select {
	case l, ok := <-lines:
		return l, ok
	case <-time.After(d):
		return "", false
	}
}
