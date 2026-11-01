// Package live publishes hub state changes to clients over Server-Sent Events.
package live

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Broker sends each published event to every connected client.
type Broker struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

// NewBroker returns a broker with no clients.
func NewBroker() *Broker {
	return &Broker{subs: map[chan string]struct{}{}}
}

// Frame formats one SSE event. Each line of a multi-line payload needs its own "data:" prefix.
func Frame(name, data string) string {
	return "event: " + name + "\n" + "data: " + strings.ReplaceAll(data, "\n", "\ndata: ") + "\n\n"
}

// Publish sends one event to every client. A client that is not keeping up misses the event
// rather than holding up the hub.
func (b *Broker) Publish(name, data string) {
	frame := Frame(name, data)
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- frame:
		default:
		}
	}
}

// Clients is the number of connected clients.
func (b *Broker) Clients() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

func (b *Broker) subscribe() chan string {
	ch := make(chan string, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// Handler serves /events. It sends the current state first, from snapshot, so a late client is not
// blank, then every event published after it connected.
func (b *Broker) Handler(snapshot func() string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		ch := b.subscribe()
		defer b.unsubscribe(ch)
		fmt.Fprint(w, snapshot())
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case frame := <-ch:
				fmt.Fprint(w, frame)
				flusher.Flush()
			}
		}
	})
}
