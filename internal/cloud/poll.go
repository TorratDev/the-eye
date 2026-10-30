package cloud

import (
	"context"
	"fmt"
	"time"
)

// PollStore is what a poll needs: reconciliation, plus the newest comment already applied per sub-issue.
type PollStore interface {
	ClaimStore
	CloudSeen(number int) (string, error)
	SetCloudSeen(number int, at string) error
}

// Fetcher reads one issue's comments. FetchComments is the GitHub one.
type Fetcher func(repo string, issue int) ([]Comment, error)

// Poller reads each open sub-issue's comments on every tick and reconciles only the ones it has not applied yet.
type Poller struct {
	Store  PollStore
	Repo   string
	Fetch  Fetcher
	Issues func() ([]int, error) // open sub-issues of the epic

	// OnOutcome is called with what each sub-issue's new comments did. Optional.
	OnOutcome func(issue int, out Outcome)
	// OnError is called for a sub-issue that failed this tick. The other sub-issues still run. Optional.
	OnError func(issue int, err error)
}

// Tick runs one pass over the epic's open sub-issues.
func (p *Poller) Tick() error {
	issues, err := p.Issues()
	if err != nil {
		return err
	}
	for _, n := range issues {
		if err := p.tickIssue(n); err != nil && p.OnError != nil {
			p.OnError(n, err)
		}
	}
	return nil
}

func (p *Poller) tickIssue(n int) error {
	seen, err := p.Store.CloudSeen(n)
	if err != nil {
		return err
	}
	comments, err := p.Fetch(p.Repo, n)
	if err != nil {
		return err
	}
	fresh, newest, err := newerThan(comments, seen)
	if err != nil {
		return fmt.Errorf("#%d: %w", n, err)
	}
	if len(fresh) == 0 {
		return nil
	}
	out, reconcileErr := Reconcile(p.Store, n, fresh)
	// advance past the comments even when one was malformed, so the same error is not raised every tick
	if err := p.Store.SetCloudSeen(n, newest); err != nil {
		return err
	}
	if p.OnOutcome != nil {
		p.OnOutcome(n, out)
	}
	return reconcileErr
}

// newerThan keeps the comments created after seen and returns the newest creation time among them.
// An empty seen keeps every comment.
func newerThan(comments []Comment, seen string) ([]Comment, string, error) {
	var cutoff time.Time
	if seen != "" {
		t, err := time.Parse(time.RFC3339, seen)
		if err != nil {
			return nil, "", fmt.Errorf("stored comment time %q: %w", seen, err)
		}
		cutoff = t
	}
	var fresh []Comment
	var newest time.Time
	for _, c := range comments {
		t, err := time.Parse(time.RFC3339, c.CreatedAt)
		if err != nil {
			return nil, "", fmt.Errorf("comment time %q: %w", c.CreatedAt, err)
		}
		if !t.After(cutoff) {
			continue
		}
		fresh = append(fresh, c)
		if t.After(newest) {
			newest = t
		}
	}
	if newest.IsZero() {
		return nil, seen, nil
	}
	return fresh, newest.UTC().Format(time.RFC3339), nil
}

// Run ticks every interval until ctx is done.
func (p *Poller) Run(ctx context.Context, interval time.Duration) error {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := p.Tick(); err != nil && p.OnError != nil {
			p.OnError(0, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}
