package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
)

// A permission prompt gets Allow and Deny buttons that reach Answer; an escalation links to the
// orchestrator's page; a tool error and a cloud finding get nothing.
func TestAnswerAlerts(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, e := range []hub.Event{
		{Hook: "Notification", Role: "executor", Pane: "wH:p6", Payload: `{"notification_type":"permission_prompt","message":"Claude needs your permission to use Write"}`},
		{Hook: "Notification", Role: "orchestrator", Pane: "wH:p2", Payload: `{"message":"ESCALATE #22: which API?"}`},
		{Hook: "PostToolUseFailure", Role: "executor", Pane: "wH:p7", Payload: `{"error":"exit 1"}`},
		{Hook: "Notification", Role: "cloud", Pane: "cloud-9", Payload: `{"message":"#9 cloud BLOCKED: x"}`},
	} {
		e.ReceivedAt = time.Now()
		if err := st.RecordEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	alerts, err := Alerts(st, 10)
	if err != nil {
		t.Fatal(err)
	}
	answers := map[string]string{}
	for _, a := range alerts {
		answers[a.Pane] = a.Answer
	}
	if want := map[string]string{"wH:p6": "keys", "wH:p2": "page", "wH:p7": "", "cloud-9": ""}; len(answers) != 4 || answers["wH:p6"] != want["wH:p6"] || answers["wH:p2"] != want["wH:p2"] || answers["wH:p7"] != "" || answers["cloud-9"] != "" {
		t.Fatalf("answers = %v", answers)
	}

	var got []string
	srv := httptest.NewServer(Handler(st, live.NewBroker(), nil, nil, Actions{Answer: func(pane string, allow bool) error {
		got = append(got, pane+map[bool]string{true: " allow", false: " deny"}[allow])
		return nil
	}}))
	defer srv.Close()
	page := get(t, srv.URL+"/")
	for _, want := range []string{`hx-post="/sessions/wH:p6/answer"`, `hx-vals='{"choice":"allow"}'`, `<a href="/sessions/wH:p2">Answer</a>`} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	answer := func(choice string, header map[string]string) int {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/sessions/wH:p6/answer", strings.NewReader(url.Values{"choice": {choice}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range header {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return res.StatusCode
	}
	own := map[string]string{"HX-Request": "true", "Origin": srv.URL}
	if code := answer("allow", own); code != 200 {
		t.Errorf("allow: %d", code)
	}
	if code := answer("deny", own); code != 200 {
		t.Errorf("deny: %d", code)
	}
	if code := answer("rm -rf", own); code != http.StatusBadRequest {
		t.Errorf("bad choice: %d", code)
	}
	if code := answer("allow", map[string]string{"Origin": "https://evil.example"}); code != http.StatusForbidden {
		t.Errorf("foreign page: %d", code)
	}
	if strings.Join(got, ",") != "wH:p6 allow,wH:p6 deny" {
		t.Fatalf("answers reached: %q", got)
	}
}
