package ui

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/transcript"
)

// maxTurns and maxText bound what one session page renders.
const (
	maxTurns = 200
	maxText  = 4000
)

var turnsFragment = template.Must(template.New("turns").Funcs(template.FuncMap{"clip": clip}).Parse(
	`{{if .Missing}}<p>no transcript yet</p>{{else}}{{if .Skipped}}<p>{{.Skipped}} earlier turns not shown</p>{{end}}
{{range .Turns}}{{if eq .Kind "tool"}}<details class="tool"><summary>{{.Tool}}</summary><pre>{{clip .Text}}</pre></details>
{{else if eq .Kind "result"}}<details class="result{{if .Error}} error{{end}}"><summary>{{if .Error}}error{{else}}result{{end}}</summary><pre>{{clip .Text}}</pre></details>
{{else}}<div class="{{.Kind}}"><b>{{.Kind}}</b><pre>{{clip .Text}}</pre></div>
{{end}}{{end}}{{end}}`))

var sessionPage = template.Must(template.New("session").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Session {{.Pane}}</title>
<script src="https://unpkg.com/htmx.org@1.9.12"></script>
<script src="https://unpkg.com/htmx.org@1.9.12/dist/ext/sse.js"></script>
<style>pre{white-space:pre-wrap;margin:.25em 0}.user{background:#eef}.error summary{color:#b00}</style>
</head><body>
<p><a href="/">All sessions</a></p>
<h1>Session {{.Pane}}</h1>
<div hx-ext="sse" sse-connect="/events">
<div id="turns" hx-get="/sessions/{{.Pane}}/turns" hx-trigger="sse:state">{{.Turns}}</div>
</div>
</body></html>`))

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

// sessionHandlers serves one session's page and its turns fragment. The page re-fetches the
// fragment on every live update, which a hook event (a tool call or a finished turn) triggers.
func sessionHandlers(mux *http.ServeMux, st *hub.Store) {
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
