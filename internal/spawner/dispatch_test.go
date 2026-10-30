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
