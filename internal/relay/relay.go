// Package relay forwards the hub's /channel stream into the orchestrator session as channel
// events, so the hub's messages reach it mid-turn instead of waiting for its pane to go idle.
package relay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// Method is the notification that delivers a channel event into a claude session.
const Method = "notifications/claude/channel"

// ToSession returns a forward function that sends each event's data to every client of s.
func ToSession(s *server.MCPServer) func(Event) {
	return func(e Event) {
		s.SendNotificationToAllClients(Method, map[string]any{"content": e.Data})
	}
}

// Event is one SSE event from the hub.
type Event struct {
	Name string
	Data string
}

// ReadEvents parses SSE frames from r and calls fn for each, in order.
func ReadEvents(r io.Reader, fn func(Event)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var name string
	var data []string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if len(data) > 0 {
				fn(Event{Name: name, Data: strings.Join(data, "\n")})
			}
			name, data = "", nil
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = append(data, strings.TrimPrefix(line, "data: "))
		}
	}
	return sc.Err()
}

// Run connects to hubURL/channel, forwards each event and reconnects whenever the stream ends or
// fails, until ctx is done. The hub sends the current state on connect, so a reconnect resumes
// with the latest view. The wait between attempts starts at retry and doubles up to 30 seconds.
func Run(ctx context.Context, hubURL string, retry time.Duration, forward func(Event)) error {
	wait := retry
	for {
		connect(ctx, hubURL, forward) // a failed or ended stream is retried below
		if ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		if wait *= 2; wait > 30*time.Second {
			wait = 30 * time.Second
		}
	}
}

func connect(ctx context.Context, hubURL string, forward func(Event)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hubURL+"/channel", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("hub returned %s", res.Status)
	}
	return ReadEvents(res.Body, forward)
}
