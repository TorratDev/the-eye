package transcript

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/hub"
)

// The fixture has the entry shapes seen in the #99 live run, plus a cut-off last line.
func TestReadSession(t *testing.T) {
	turns, err := Read("testdata/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, tr := range turns {
		kinds = append(kinds, tr.Kind)
	}
	if want := []string{"user", "tool", "result", "tool", "result", "assistant"}; !slices.Equal(kinds, want) {
		t.Fatalf("kinds = %q, want %q", kinds, want)
	}
	if turns[0].Text != "Epic #7." {
		t.Errorf("user turn = %q", turns[0].Text)
	}
	if turns[1].Tool != "Bash" || !strings.Contains(turns[1].Text, "gh issue view 7") {
		t.Errorf("tool call = %+v", turns[1])
	}
	if !turns[2].Error || !strings.Contains(turns[2].Text, "no such issue") {
		t.Errorf("error result = %+v", turns[2])
	}
	if turns[4].Text != "reported" || turns[4].Error {
		t.Errorf("block result = %+v", turns[4])
	}
	if got := LastAssistant(turns); got != "Decision for the hub:\n\n```\nPOINTS #8: A= B= C=\n```" {
		t.Errorf("last assistant = %q", got)
	}
}

func TestBrokenEntryIsAnError(t *testing.T) {
	if _, err := Parse(strings.NewReader("{\"type\":\"user\"}\nnot json\n")); err == nil {
		t.Error("Parse succeeded on a broken entry")
	}
}

// PathFor takes the newest absolute .jsonl transcript_path for the pane.
func TestPathFor(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, payload := range []string{
		`{"transcript_path":"/home/u/.claude/projects/x/old.jsonl"}`,
		`{"transcript_path":"/home/u/.claude/projects/x/new.jsonl"}`,
		`{"transcript_path":"/etc/passwd"}`,
		`{}`,
	} {
		if err := st.RecordEvent(hub.Event{ReceivedAt: time.Now(), Hook: "PostToolUse", Role: "executor", Pane: "p1", Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := PathFor(st, "p1"); err != nil || got != "/home/u/.claude/projects/x/new.jsonl" {
		t.Errorf("PathFor = %q, %v", got, err)
	}
	if got, _ := PathFor(st, "p2"); got != "" {
		t.Errorf("PathFor(unknown pane) = %q", got)
	}
}
