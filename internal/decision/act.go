package decision

import (
	"fmt"
	"strings"

	"github.com/TorratDev/the-eye/internal/spawner"
)

// Deps are the hub's effects a decision needs. The hub supplies the real ones; tests supply fakes.
type Deps struct {
	Claim            func(issue int, owner string) (bool, error)
	Worktree         func(issue int, slug string) (string, error)
	FreeExecutorPane func() (string, error) // "" when every executor is busy
	InvestigatorPane func(letter string) (string, error)
	Send             func(pane, text string) error // SendWhenIdle in production
	SetTask          func(issue int, pane, worktree string) error
	CreateIssue      func(title, body string) (int, error)
	Escalate         func(text string) error
	Assign           func(issue int, pane string) // optional: told which sub-issue each pane was sent
}

// Result says what Act did with one reply. Failures are kept, never dropped.
type Result struct {
	Dispatched []string // one line per message sent or issue created
	Queued     []string // TASK blocks held back by the batch cap, to run in a later batch
	Failed     []string // decisions that could not be carried out, with the reason
}

// Act carries out the decisions in an orchestrator reply. A TASK beyond the batch cap is queued,
// not sent. A decision that fails is recorded in Failed and the rest still run.
func Act(reply string, d Deps) (Result, error) {
	acts, err := Route(reply, func(string) string { return "pending" })
	if err != nil {
		return Result{}, err
	}
	var res Result
	var tasks []Action
	for _, a := range acts {
		if a.Kind == "execute" {
			tasks = append(tasks, a)
		}
	}
	start, _ := spawner.Batch(issuesOf(tasks))
	startSet := map[int]bool{}
	for _, n := range start {
		startSet[n] = true
	}
	for _, a := range acts {
		switch a.Kind {
		case "investigate":
			for _, inv := range a.Investigate {
				if err := dispatchInvestigator(d, inv, &res); err != nil {
					res.Failed = append(res.Failed, fmt.Sprintf("POINTS #%d %s: %v", inv.Issue, inv.Letter, err))
				}
			}
		case "execute":
			ex := a.Execute
			if !startSet[ex.Issue] {
				res.Queued = append(res.Queued, fmt.Sprintf("TASK #%d", ex.Issue))
				continue
			}
			if err := dispatchExecutor(d, ex, &res); err != nil {
				res.Failed = append(res.Failed, fmt.Sprintf("TASK #%d: %v", ex.Issue, err))
			}
		case "followup":
			f := a.Followup
			n, err := d.CreateIssue(f.Fields["title"], followupBody(f))
			if err != nil {
				res.Failed = append(res.Failed, fmt.Sprintf("FOLLOWUP #%d: %v", f.Issue, err))
				continue
			}
			res.Dispatched = append(res.Dispatched, fmt.Sprintf("created #%d from FOLLOWUP #%d", n, f.Issue))
		case "escalate":
			if err := d.Escalate(a.Escalate); err != nil {
				res.Failed = append(res.Failed, fmt.Sprintf("ESCALATE: %v", err))
				continue
			}
			res.Dispatched = append(res.Dispatched, "escalated "+a.Escalate)
		}
	}
	return res, nil
}

func issuesOf(tasks []Action) []int {
	var out []int
	for _, t := range tasks {
		out = append(out, t.Execute.Issue)
	}
	return out
}

// dispatchExecutor claims the sub-issue, makes its worktree and sends the message to an idle executor.
// A claim that is already held is a failure: another session has the sub-issue.
func dispatchExecutor(d Deps, ex *spawner.ExecutorDispatch, res *Result) error {
	ok, err := d.Claim(ex.Issue, "orchestrator")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("already claimed")
	}
	slug := strings.TrimPrefix(ex.Branch, fmt.Sprintf("%d-", ex.Issue))
	wt, err := d.Worktree(ex.Issue, slug)
	if err != nil {
		return err
	}
	ex.Worktree = wt
	pane, err := d.FreeExecutorPane()
	if err != nil {
		return err
	}
	if pane == "" {
		return fmt.Errorf("no idle executor pane; send it in a later batch")
	}
	msg, err := ex.Message()
	if err != nil {
		return err
	}
	if err := d.Send(pane, msg); err != nil {
		return err
	}
	if err := d.SetTask(ex.Issue, pane, wt); err != nil {
		return err
	}
	if d.Assign != nil {
		d.Assign(ex.Issue, pane)
	}
	res.Dispatched = append(res.Dispatched, fmt.Sprintf("TASK #%d to %s", ex.Issue, pane))
	return nil
}

func dispatchInvestigator(d Deps, inv spawner.InvestigatorDispatch, res *Result) error {
	pane, err := d.InvestigatorPane(inv.Letter)
	if err != nil {
		return err
	}
	msg, err := inv.Message()
	if err != nil {
		return err
	}
	if err := d.Send(pane, msg); err != nil {
		return err
	}
	if d.Assign != nil {
		d.Assign(inv.Issue, pane)
	}
	res.Dispatched = append(res.Dispatched, fmt.Sprintf("POINTS #%d %s to %s", inv.Issue, inv.Letter, pane))
	return nil
}

func followupBody(f *Followup) string {
	var b strings.Builder
	for _, k := range []string{"done", "remaining", "reason", "deps"} {
		if v := f.Fields[k]; v != "" {
			fmt.Fprintf(&b, "**%s:** %s\n\n", k, v)
		}
	}
	fmt.Fprintf(&b, "Split from #%d by the orchestrator.", f.Issue)
	return b.String()
}
