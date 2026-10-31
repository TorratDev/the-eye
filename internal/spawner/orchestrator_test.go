package spawner

import (
	"errors"
	"strings"
	"testing"
)

func TestParseLogin(t *testing.T) {
	cases := map[string]string{
		"not logged in": `{"loggedIn":false,"authMethod":"claude.ai"}`,
		"api key":       `{"loggedIn":true,"authMethod":"api_key"}`,
	}
	for name, raw := range cases {
		if err := parseLogin([]byte(raw)); err == nil {
			t.Errorf("%s: accepted, want error", name)
		}
	}
	if err := parseLogin([]byte(`{"loggedIn":true,"authMethod":"claude.ai"}`)); err != nil {
		t.Errorf("claude.ai login refused: %v", err)
	}
}

// Without the login the launcher reports the missing login and types nothing into the pane.
func TestNoLoginStartsNothing(t *testing.T) {
	typed := false
	err := startOrchestrator(
		func() error { return errors.New("not logged in: run `claude auth login`") },
		false, "orchestrator", "prompt", "",
		func(pane, cmd string) error { typed = true; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "claude auth login") {
		t.Errorf("err = %v, want the missing login named", err)
	}
	if typed {
		t.Error("launch line was typed without a login")
	}
}

func TestChannelFlagOnlyWhenAvailable(t *testing.T) {
	var got string
	run := func(pane, cmd string) error { got = cmd; return nil }
	ok := func() error { return nil }

	if err := startOrchestrator(ok, true, "orchestrator", "p", "", run); err != nil || !strings.HasSuffix(got, ChannelFlag) {
		t.Errorf("with channels: cmd = %q, err = %v", got, err)
	}
	if err := startOrchestrator(ok, false, "orchestrator", "p", "", run); err != nil || strings.Contains(got, ChannelFlag) {
		t.Errorf("without channels: cmd = %q, err = %v", got, err)
	}
}
