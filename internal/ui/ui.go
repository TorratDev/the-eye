// Package ui serves the watcher page: one row per session, updated live from /events.
// It has no controls; every action stays with the orchestrator and the hub.
package ui

import (
	"bytes"
	"html/template"
	"net/http"
	"sort"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/ingest"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/session"
)

// Row is one session as the page shows it.
type Row struct {
	Role  string
	Pane  string
	State string
	Issue int // sub-issue running in the pane, 0 if none
	At    time.Time
}

// Rows builds one row per pane from the latest stored event of each.
func Rows(st *hub.Store) ([]Row, error) {
	events, err := st.LatestPerPane()
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, e := range events {
		state := string(session.Apply("", session.Event{Hook: e.Hook, Role: e.Role, Pane: e.Pane}))
		if state == "" {
			state = "unknown"
		}
		issue, err := st.SubIssueForPane(e.Pane)
		if err != nil {
			return nil, err
		}
		rows = append(rows, Row{Role: e.Role, Pane: e.Pane, State: state, Issue: issue, At: e.ReceivedAt})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Pane < rows[j].Pane })
	return rows, nil
}

var fragment = template.Must(template.New("sessions").Parse(`<table>
<thead><tr><th>Role</th><th>Pane</th><th>State</th><th>Sub-issue</th><th>Last event</th></tr></thead>
<tbody>{{range .}}<tr><td>{{.Role}}</td><td>{{.Pane}}</td><td>{{.State}}</td><td>{{if .Issue}}#{{.Issue}}{{else}}-{{end}}</td><td>{{.At.Format "15:04:05"}}</td></tr>{{else}}<tr><td colspan="5">no sessions yet</td></tr>{{end}}</tbody>
</table>`))

// SessionsFragment is the table the page and each live update carry.
func SessionsFragment(st *hub.Store) (string, error) {
	rows, err := Rows(st)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := fragment.Execute(&buf, rows); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Hub sessions</title>
<script src="https://unpkg.com/htmx.org@1.9.12"></script>
<script src="https://unpkg.com/htmx.org@1.9.12/dist/ext/sse.js"></script>
</head><body>
<h1>Sessions</h1>
<div id="sessions" hx-ext="sse" sse-connect="/events" sse-swap="state">{{.}}</div>
</body></html>`))

// Handler serves the page, the hook endpoint and the live stream. A stored hook event is
// published to every connected page as a new sessions table.
func Handler(st *hub.Store, b *live.Broker) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		frag, err := SessionsFragment(st)
		if err != nil {
			http.Error(w, "state unavailable", http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		if err := page.Execute(&buf, template.HTML(frag)); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(buf.Bytes())
	})
	mux.Handle("POST /hooks", ingest.Handler(st, func(hub.Event) {
		if frag, err := SessionsFragment(st); err == nil {
			b.Publish("state", frag)
		}
	}))
	mux.Handle("GET /events", b.Handler(func() string {
		frag, err := SessionsFragment(st)
		if err != nil {
			return ""
		}
		return live.Frame("state", frag)
	}))
	return mux
}
