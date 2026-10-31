package spawner

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/TorratDev/the-eye/internal/herdr"
)

func TestBatchCap(t *testing.T) {
	start, queue := Batch([]int{1, 2, 3, 4})
	if !reflect.DeepEqual(start, []int{1, 2, 3}) || !reflect.DeepEqual(queue, []int{4}) {
		t.Errorf("Batch(4 ready) = %v, %v; want [1 2 3], [4]", start, queue)
	}
}

func TestSendWhenIdle(t *testing.T) {
	// busy, then idle: the text is sent once, after the pane is idle
	statuses := []string{"working", "idle"}
	sent := 0
	get := func() (herdr.Pane, error) {
		s := statuses[0]
		statuses = statuses[1:]
		return herdr.Pane{ID: "p1", AgentStatus: s}, nil
	}
	if err := sendWhenIdle(get, func() error { sent++; return nil }, time.Second, 0); err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Errorf("sent %d times, want 1", sent)
	}

	// always busy: refused, nothing sent
	sent = 0
	busy := func() (herdr.Pane, error) { return herdr.Pane{ID: "p1", AgentStatus: "working"}, nil }
	if err := sendWhenIdle(busy, func() error { sent++; return nil }, 0, 0); err == nil || sent != 0 {
		t.Errorf("busy pane: err=%v sent=%d, want refusal and no send", err, sent)
	}

	// herdr error is returned
	fail := func() (herdr.Pane, error) { return herdr.Pane{}, errors.New("boom") }
	if err := sendWhenIdle(fail, func() error { return nil }, 0, 0); err == nil {
		t.Error("herdr error swallowed")
	}
}

// A blocked session gets Enter to allow or Esc to deny; any other state gets no key at all.
func TestAnswerPrompt(t *testing.T) {
	for _, c := range []struct {
		status string
		allow  bool
		want   string
	}{{"blocked", true, "enter"}, {"blocked", false, "esc"}, {"idle", true, ""}, {"working", false, ""}, {"unknown", true, ""}} {
		var sent string
		err := answerPrompt(func() (herdr.Pane, error) { return herdr.Pane{ID: "p6", AgentStatus: c.status}, nil },
			func(key string) error { sent = key; return nil }, c.allow)
		if sent != c.want || (c.want == "") != (err != nil) {
			t.Errorf("%s allow=%v: sent %q, err %v", c.status, c.allow, sent, err)
		}
	}
}

// A finished turn herdr still shows as done is ready for input, as idle is; blocked and unknown are not.
func TestSendWhenDone(t *testing.T) {
	for status, want := range map[string]bool{"done": true, "idle": true, "blocked": false, "unknown": false} {
		sent := false
		get := func() (herdr.Pane, error) { return herdr.Pane{ID: "p1", AgentStatus: status}, nil }
		err := sendWhenIdle(get, func() error { sent = true; return nil }, 5*time.Millisecond, time.Millisecond)
		if sent != want || (err == nil) != want {
			t.Errorf("%s: sent %v, err %v", status, sent, err)
		}
	}
}
