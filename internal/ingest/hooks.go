// Package ingest receives Claude Code hook events for the hub.
package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
)

// hooks are the events the hub stores. PostToolUseFailure is stored too: the state model reads it as error.
var hooks = []string{"Stop", "Notification", "PostToolUse", "PostToolUseFailure"}

// roles are the roles a session can be launched as.
var roles = []string{"orchestrator", "investigator", "executor", "reviewer"}

// maxBody bounds one hook payload.
const maxBody = 1 << 20

// Handler returns the POST /hooks endpoint. The hook command forwards the role and pane the
// session was launched with, from HUB_ROLE and HUB_PANE, in the X-Hub-Role and X-Hub-Pane headers.
// onEvent, if set, is called with each stored event, so live clients see it.
func Handler(st *hub.Store, onEvent func(hub.Event)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil || len(raw) > maxBody {
			http.Error(w, "unreadable or too large body", http.StatusBadRequest)
			return
		}
		e, err := parse(r.Header.Get("X-Hub-Role"), r.Header.Get("X-Hub-Pane"), raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		e.ReceivedAt = time.Now()
		if err := st.RecordEvent(e); err != nil {
			http.Error(w, "store failed", http.StatusInternalServerError)
			return
		}
		if onEvent != nil {
			onEvent(e)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// parse checks one hook event and turns it into a stored event.
func parse(role, pane string, raw []byte) (hub.Event, error) {
	if !slices.Contains(roles, role) {
		return hub.Event{}, fmt.Errorf("unknown role %q", role)
	}
	if pane == "" {
		return hub.Event{}, fmt.Errorf("missing pane")
	}
	var body struct {
		HookEventName string `json:"hook_event_name"`
		SessionID     string `json:"session_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return hub.Event{}, fmt.Errorf("body is not JSON: %w", err)
	}
	if !slices.Contains(hooks, body.HookEventName) {
		return hub.Event{}, fmt.Errorf("hook %q is not stored", body.HookEventName)
	}
	return hub.Event{Hook: body.HookEventName, Role: role, Pane: pane, Session: body.SessionID, Payload: string(raw)}, nil
}
