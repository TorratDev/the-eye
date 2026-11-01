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

// SendWhenIdle sends text to pane's session once it reports itself ready for input: idle, or done
// (a finished turn). A session that has not reported yet is not ready. If it stays busy past
// timeout it returns an error and sends nothing, so the caller can retry later.
func SendWhenIdle(pane, text string, timeout time.Duration) error {
	return sendWhenIdle(
		func() (herdr.Terminal, error) { return herdr.Get(pane) },
		func() error { return herdr.Send(pane, text) },
		timeout, time.Second,
	)
}

// sendWhenIdle is SendWhenIdle with the hc calls and poll interval injected.
func sendWhenIdle(get func() (herdr.Terminal, error), send func() error, timeout, poll time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		t, err := get()
		if err != nil {
			return err
		}
		if s := t.Status.State; s == "idle" || s == "done" {
			return send()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("pane %s is %s, not idle after %s", t.Name, stateOf(t), timeout)
		}
		time.Sleep(poll)
	}
}

// stateOf is the session's reported state, or "unknown" before it has reported.
func stateOf(t herdr.Terminal) string {
	if t.Status.State == "" {
		return "unknown"
	}
	return t.Status.State
}

// AnswerPrompt answers the permission dialog a session waits on: Enter takes the dialog's default
// (allow), Esc denies. It refuses unless the session reports itself blocked, so the key never
// lands in a running session or a shell.
func AnswerPrompt(pane string, allow bool) error {
	return answerPrompt(func() (herdr.Terminal, error) { return herdr.Get(pane) }, func(key string) error { return herdr.Keys(pane, key) }, allow)
}

func answerPrompt(get func() (herdr.Terminal, error), keys func(key string) error, allow bool) error {
	t, err := get()
	if err != nil {
		return err
	}
	if t.Status.State != "blocked" {
		return fmt.Errorf("pane %s is %s, not waiting on a prompt", t.Name, stateOf(t))
	}
	if allow {
		return keys("enter")
	}
	return keys("esc")
}
