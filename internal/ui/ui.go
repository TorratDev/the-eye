// Package ui serves the watcher page (one row per session) and a page per session with its
// transcript, both updated live from /events. A session page can send its session a message.
package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/ingest"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/report"
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

// Alert is one tool failure or permission prompt in the errors panel.
type Alert struct {
	At     time.Time
	Role   string
	Pane   string
	Kind   string // "tool error" or "prompt"
	Detail string
	Answer string // "keys": allow or deny a permission prompt; "page": answer on the session page; "" for none
}

// Alerts builds the errors panel: the newest tool failures and prompts, newest first.
func Alerts(st *hub.Store, limit int) ([]Alert, error) {
	events, err := st.RecentAlerts(limit)
	if err != nil {
		return nil, err
	}
	var out []Alert
	for _, e := range events {
		a := Alert{At: e.ReceivedAt, Role: e.Role, Pane: e.Pane, Kind: "prompt", Detail: detail(e.Payload)}
		switch {
		case e.Hook == "PostToolUseFailure":
			a.Kind = "tool error"
		case notificationType(e.Payload) == "permission_prompt":
			a.Answer = "keys"
		case e.Role == "orchestrator" && strings.HasPrefix(a.Detail, "ESCALATE "):
			a.Answer = "page"
		}
		out = append(out, a)
	}
	return out, nil
}

func notificationType(payload string) string {
	var p struct {
		Type string `json:"notification_type"`
	}
	json.Unmarshal([]byte(payload), &p)
	return p.Type
}

// detail takes the readable part of a hook payload: the error, the prompt message or the tool name.
func detail(payload string) string {
	var p map[string]any
	if json.Unmarshal([]byte(payload), &p) != nil {
		return ""
	}
	for _, key := range []string{"error", "message", "tool_name"} {
		if v, ok := p[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// Progress is the count of sub-issues in each state, with their numbers.
func Progress(st *hub.Store) (map[string][]int, error) {
	return st.TaskNumbersByState()
}

var dashboard = template.Must(template.New("dashboard").Parse(`<h2>Errors and prompts</h2>
<table>
<thead><tr><th>Time</th><th>Role</th><th>Pane</th><th>Kind</th><th>Detail</th><th>Answer</th></tr></thead>
<tbody>{{range .Alerts}}<tr><td>{{.At.Format "15:04:05"}}</td><td>{{.Role}}</td><td>{{.Pane}}</td><td>{{.Kind}}</td><td>{{.Detail}}</td><td>
{{- if eq .Answer "keys"}}<span hx-target="this" hx-swap="innerHTML" hx-on::response-error="this.textContent=event.detail.xhr.responseText">
<button hx-post="/sessions/{{.Pane}}/answer" hx-vals='{"choice":"allow"}'>Allow</button>
<button hx-post="/sessions/{{.Pane}}/answer" hx-vals='{"choice":"deny"}'>Deny</button></span>
{{- else if eq .Answer "page"}}<a href="/sessions/{{.Pane}}">Answer</a>{{end -}}
</td></tr>{{else}}<tr><td colspan="6">none</td></tr>{{end}}</tbody>
</table>
<h2>Sessions</h2>
<table>
<thead><tr><th>Role</th><th>Pane</th><th>State</th><th>Sub-issue</th><th>Last event</th></tr></thead>
<tbody>{{range .Rows}}<tr><td>{{.Role}}</td><td><a href="/sessions/{{.Pane}}">{{.Pane}}</a></td><td>{{.State}}</td><td>{{if .Issue}}#{{.Issue}}{{else}}-{{end}}</td><td>{{.At.Format "15:04:05"}}</td></tr>{{else}}<tr><td colspan="5">no sessions yet</td></tr>{{end}}</tbody>
</table>
<h2>Progress</h2>
<ul>{{range .Progress}}<li>{{.State}}: {{.Issues}}</li>{{else}}<li>no sub-issues tracked</li>{{end}}</ul>`))

type progressLine struct {
	State  string
	Issues string
}

// SessionsFragment is what the page and each live update carry: errors, sessions and progress.
func SessionsFragment(st *hub.Store) (string, error) {
	rows, err := Rows(st)
	if err != nil {
		return "", err
	}
	alerts, err := Alerts(st, 20)
	if err != nil {
		return "", err
	}
	byState, err := Progress(st)
	if err != nil {
		return "", err
	}
	var lines []progressLine
	states := make([]string, 0, len(byState))
	for s := range byState {
		states = append(states, s)
	}
	sort.Strings(states)
	for _, s := range states {
		var nums []string
		for _, n := range byState[s] {
			nums = append(nums, "#"+strconv.Itoa(n))
		}
		lines = append(lines, progressLine{State: s, Issues: strings.Join(nums, ", ")})
	}
	var buf bytes.Buffer
	err = dashboard.Execute(&buf, struct {
		Rows     []Row
		Alerts   []Alert
		Progress []progressLine
	}{rows, alerts, lines})
	if err != nil {
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
<div id="dashboard" hx-ext="sse" sse-connect="/events" sse-swap="state">{{.}}</div>
</body></html>`))

// finishFromFinalLine ends the task in an executor's or reviewer's pane from the final line of its
// Stop event: DONE or BLOCKED for an executor (the fallback when it skipped the report tool), the
// verdict for a reviewer. That frees the pane.
func finishFromFinalLine(st *hub.Store, e hub.Event) {
	if e.Hook != "Stop" || (e.Role != "executor" && e.Role != "reviewer") {
		return
	}
	var p struct {
		Reply string `json:"last_assistant_message"`
	}
	json.Unmarshal([]byte(e.Payload), &p)
	f, _, ok := session.FinalIn(e.Role, p.Reply)
	if !ok {
		return
	}
	if err := st.FinishPaneTask(e.Pane, session.TaskState(f.Kind)); err != nil {
		log.Printf("hub: finish task in %s: %v", e.Pane, err)
	}
}

// Handler serves the page, the hook endpoint, the live stream and the report endpoint. A stored hook
// event is published to every connected page as a new sessions table, then passed to onEvent if set.
// c posts report comments; with nil, POST /report refuses. act holds what the page may do to a
// session; a nil action refuses.
func Handler(st *hub.Store, b *live.Broker, c report.Commenter, onEvent func(hub.Event), act Actions) http.Handler {
	mux := http.NewServeMux()
	sessionHandlers(mux, st, act)
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
	mux.HandleFunc("POST /report", func(w http.ResponseWriter, r *http.Request) {
		if c == nil {
			http.Error(w, "no GitHub commenter configured", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Issue   int    `json:"issue"`
			Status  string `json:"status"`
			Summary string `json:"summary"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
			http.Error(w, "body is not a report", http.StatusBadRequest)
			return
		}
		if err := report.Submit(st, c, body.Issue, r.Header.Get("X-Hub-Role"), body.Status, body.Summary); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("POST /hooks", ingest.Handler(st, func(e hub.Event) {
		finishFromFinalLine(st, e)
		if frag, err := SessionsFragment(st); err == nil {
			b.Publish("state", frag)
		}
		if onEvent != nil {
			onEvent(e)
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
