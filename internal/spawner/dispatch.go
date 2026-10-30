package spawner

import (
	"fmt"
	"time"

	"github.com/TorratDev/the-eye/internal/herdr"
)

// MaxBatch is the most sub-issues one batch starts.
const MaxBatch = 3

// Batch splits ready sub-issues into the ones to start now and the ones to queue.
func Batch(ready []int) (start, queue []int) {
	if len(ready) <= MaxBatch {
		return ready, nil
	}
	return ready[:MaxBatch], ready[MaxBatch:]
}

// SendWhenIdle types text into pane once herdr reports it idle. If the pane stays busy past
// timeout it returns an error and sends nothing, so the caller can retry later.
func SendWhenIdle(pane, text string, timeout time.Duration) error {
	return sendWhenIdle(
		func() (herdr.Pane, error) { return herdr.PaneGet(pane) },
		func() error { return herdr.PaneRun(pane, text) },
		timeout, time.Second,
	)
}

// sendWhenIdle is SendWhenIdle with the herdr calls and poll interval injected.
func sendWhenIdle(get func() (herdr.Pane, error), send func() error, timeout, poll time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		p, err := get()
		if err != nil {
			return err
		}
		if p.AgentStatus == "idle" {
			return send()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("pane %s is %s, not idle after %s", p.ID, p.AgentStatus, timeout)
		}
		time.Sleep(poll)
	}
}
