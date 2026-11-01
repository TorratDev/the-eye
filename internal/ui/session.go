package ui

import (
	"bytes"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/transcript"
)

// maxTurns and maxText bound what one session page renders.
const (
	maxTurns = 200
	maxText  = 4000
)

var turnsFragment = template.Must(template.New("turns").Funcs(template.FuncMap{"clip": clip}).Parse(
	`{{if .Missing}}<p class="empty">no transcript yet</p>{{else}}{{if .Skipped}}<p class="muted">{{.Skipped}} earlier turns not shown</p>{{end}}
{{range .Turns}}{{if eq .Kind "tool"}}<details class="tool"><summary>{{.Tool}}</summary><pre>{{clip .Text}}</pre></details>
{{else if eq .Kind "result"}}<details class="result{{if .Error}} error{{end}}"><summary>{{if .Error}}error{{else}}result{{end}}</summary><pre>{{clip .Text}}</pre></details>
{{else}}<div class="{{.Kind}}"><b>{{.Kind}}</b><pre>{{clip .Text}}</pre></div>
{{end}}{{end}}{{end}}`))

var sessionPage = template.Must(template.New("session").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Session {{.Pane}}</title>
<script src="https://unpkg.com/htmx.org@1.9.12"></script>
<script src="https://unpkg.com/htmx.org@1.9.12/dist/ext/sse.js"></script>
` + style + `
</head><body>
<header><h1>Session {{.Pane}}</h1><a href="/">← All sessions</a></header>
<main hx-ext="sse" sse-connect="/events">
<div id="turns" class="turns" hx-get="/sessions/{{.Pane}}/turns" hx-trigger="sse:state">{{.Turns}}</div>
<form class="send" hx-post="/sessions/{{.Pane}}/send" hx-target="#sent" hx-on::after-request="if(event.detail.successful)this.reset()"
 hx-on::response-error="document.getElementById('sent').textContent=event.detail.xhr.responseText">
<label for="text" class="muted">Message</label> <input id="text" name="text" maxlength="4000" required autocomplete="off">
<button>Send</button> <span id="sent" role="status"></span>
</form>
</main>
</body></html>`))

// Actions are what the page may do to a session. Each one runs code in a pane.
type Actions struct {
	Send   func(pane, text string) error       // type a message once the session is idle
	Answer func(pane string, allow bool) error // allow or deny the permission prompt the session waits on
	// StartEpic starts the orchestrator for an epic in an hc terminal; the page's epic form.
	StartEpic func(epic int) error
}

// sessionRequest checks a request that acts on a session, and writes the refusal if it fails:
// the action must be configured, the request must come from the hub's own page, and the pane must
// be one the hub has hook events from.
func sessionRequest(w http.ResponseWriter, r *http.Request, st *hub.Store, configured bool) (string, bool) {
	// acting on a pane runs code, so only the hub's own page may do it
	if !fromOwnPage(r) {
		http.Error(w, "refused: not from the hub's page", http.StatusForbidden)
		return "", false
	}
	if !configured {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return "", false
	}
	pane := r.PathValue("pane")
	if events, err := st.Events(pane); err != nil || len(events) == 0 {
		http.Error(w, "not a session the hub knows", http.StatusNotFound)
		return "", false
	}
	return pane, true
}

// fromOwnPage accepts only htmx requests from the hub's own localhost page. A cross-site form
// cannot set HX-Request without a CORS preflight the hub never answers, and the host check stops a
// DNS-rebound name from posting as same-origin.
func fromOwnPage(r *http.Request) bool {
	if r.Header.Get("HX-Request") != "true" || !localHost(r.Host) {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

func localHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

// clip shortens long tool output so one result cannot swamp the page.
func clip(s string) string {
	if len(s) <= maxText {
		return s
	}
	return s[:maxText] + "\n… (cut)"
}

// TurnsFragment renders the newest turns of the pane's transcript.
func TurnsFragment(st *hub.Store, pane string) (string, error) {
	path, err := transcript.PathFor(st, pane)
	if err != nil {
		return "", err
	}
	data := struct {
		Missing bool
		Skipped int
		Turns   []transcript.Turn
	}{Missing: path == ""}
	if path != "" {
		turns, err := transcript.Read(path)
		if err != nil {
			return "", err
		}
		if len(turns) > maxTurns {
			data.Skipped = len(turns) - maxTurns
			turns = turns[data.Skipped:]
		}
		data.Turns = turns
	}
	var buf bytes.Buffer
	err = turnsFragment.Execute(&buf, data)
	return buf.String(), err
}

// sessionHandlers serves one session's page, its turns fragment and its send endpoint. The page
// re-fetches the fragment on every live update, which a hook event (a tool call or a finished turn) triggers.
func sessionHandlers(mux *http.ServeMux, st *hub.Store, act Actions) {
	mux.HandleFunc("POST /sessions/{pane}/answer", func(w http.ResponseWriter, r *http.Request) {
		pane, ok := sessionRequest(w, r, st, act.Answer != nil)
		if !ok {
			return
		}
		choice := r.FormValue("choice")
		if choice != "allow" && choice != "deny" {
			http.Error(w, "choice must be allow or deny", http.StatusBadRequest)
			return
		}
		if err := act.Answer(pane, choice == "allow"); err != nil {
			http.Error(w, "not answered: "+err.Error(), http.StatusConflict)
			return
		}
		w.Write([]byte(choice + " sent"))
	})
	mux.HandleFunc("POST /sessions/{pane}/send", func(w http.ResponseWriter, r *http.Request) {
		pane, ok := sessionRequest(w, r, st, act.Send != nil)
		if !ok {
			return
		}
		send := act.Send
		// shortcut: one line only, since the pane gets it as typed input; allow multi-line once herdr's paste behaviour is checked
		text := strings.Join(strings.Fields(r.FormValue("text")), " ")
		if text == "" || len(text) > maxText {
			http.Error(w, "empty or too long", http.StatusBadRequest)
			return
		}
		if err := send(pane, text); err != nil {
			http.Error(w, "not sent: "+err.Error(), http.StatusConflict)
			return
		}
		w.Write([]byte("sent"))
	})
	mux.HandleFunc("GET /sessions/{pane}/turns", func(w http.ResponseWriter, r *http.Request) {
		frag, err := TurnsFragment(st, r.PathValue("pane"))
		if err != nil {
			http.Error(w, "transcript unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(frag))
	})
	mux.HandleFunc("GET /sessions/{pane}", func(w http.ResponseWriter, r *http.Request) {
		pane := r.PathValue("pane")
		frag, err := TurnsFragment(st, pane)
		if err != nil {
			http.Error(w, "transcript unavailable", http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		if err := sessionPage.Execute(&buf, struct {
			Pane  string
			Turns template.HTML
		}{pane, template.HTML(frag)}); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(buf.Bytes())
	})
}
