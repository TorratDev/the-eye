package relay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadEventsMultiline(t *testing.T) {
	var got []Event
	err := ReadEvents(strings.NewReader("event: state\ndata: <tr>\ndata: <td>x</td>\n\nevent: alert\ndata: boom\n\n"), func(e Event) {
		got = append(got, e)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Data != "<tr>\n<td>x</td>" || got[1].Name != "alert" {
		t.Errorf("events = %+v", got)
	}
}

// A hub restart ends the stream; the relay reconnects and forwards the new connection's events.
func TestReconnectResumes(t *testing.T) {
	var conns atomic.Int32
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := conns.Add(1)
		fmt.Fprintf(w, "event: state\ndata: connection %d\n\n", n)
		if n == 1 {
			return // the hub restarts: the stream ends here
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer hub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got := make(chan string, 4)
	go Run(ctx, hub.URL, 10*time.Millisecond, func(e Event) { got <- e.Data })

	for _, want := range []string{"connection 1", "connection 2"} {
		select {
		case d := <-got:
			if d != want {
				t.Errorf("got %q, want %q", d, want)
			}
		case <-ctx.Done():
			t.Fatalf("no %q after the hub restarted", want)
		}
	}
}
