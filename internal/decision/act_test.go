package decision

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// fakes records what Act asked the hub to do.
type fakes struct {
	claimed   map[int]bool
	sent      []string // "pane: first line"
	tasks     []string // "issue pane worktree"
	issues    []string
	escalated []string
	idle      string
}

func newFakes() *fakes {
	return &fakes{claimed: map[int]bool{}, idle: "executor-1"}
}

func (f *fakes) deps() Deps {
	return Deps{
		Claim: func(issue int, _ string) (bool, error) {
			if f.claimed[issue] {
				return false, nil
			}
			f.claimed[issue] = true
			return true, nil
		},
		Worktree: func(issue int, slug string) (string, error) {
			return "/wt/" + slugOf(issue, slug), nil
		},
		FreeExecutorPane: func() (string, error) { return f.idle, nil },
		InvestigatorPane: func(letter string) (string, error) { return "investigator-" + strings.ToLower(letter), nil },
		Send: func(pane, text string) error {
			f.sent = append(f.sent, pane+": "+strings.SplitN(text, "\n", 2)[0])
			return nil
		},
		SetTask: func(issue int, pane, wt string) error {
			f.tasks = append(f.tasks, strings.Join([]string{strconv.Itoa(issue), pane, wt}, " "))
			return nil
		},
		CreateIssue: func(title, body string) (int, error) {
			f.issues = append(f.issues, title)
			return 90 + len(f.issues), nil
		},
		Escalate: func(text string) error {
			f.escalated = append(f.escalated, text)
			return nil
		},
	}
}

func slugOf(issue int, slug string) string { return strconv.Itoa(issue) + "-" + slug }


// A TASK block becomes one claim, one worktree and one message to an idle executor.
func TestTaskDispatched(t *testing.T) {
	f := newFakes()
	reply := "TASK #22: slug=dispatch | objective=build | files=a.go | verify=go test | evidence=output"
	res, err := Act(reply, f.deps())
	if err != nil {
		t.Fatal(err)
	}
	if !f.claimed[22] {
		t.Error("sub-issue not claimed")
	}
	if len(f.sent) != 1 || !strings.HasPrefix(f.sent[0], "executor-1: role: executor") {
		t.Errorf("sent = %v, want one executor message", f.sent)
	}
	if len(f.tasks) != 1 || !strings.Contains(f.tasks[0], "/wt/22-dispatch") {
		t.Errorf("tasks = %v, want the worktree recorded", f.tasks)
	}
	if len(res.Failed) != 0 {
		t.Errorf("failed = %v", res.Failed)
	}
}

// A sub-issue another session holds is not dispatched, and the failure is kept.
func TestClaimedTaskRefused(t *testing.T) {
	f := newFakes()
	f.claimed[22] = true
	res, err := Act("TASK #22: slug=x | objective=y | files=a.go | verify=z | evidence=e", f.deps())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.sent) != 0 {
		t.Errorf("message sent for a claimed sub-issue: %v", f.sent)
	}
	if len(res.Failed) != 1 || !strings.Contains(res.Failed[0], "already claimed") {
		t.Errorf("failed = %v, want the claim refusal", res.Failed)
	}
}

// Four TASK blocks start three and queue the fourth.
func TestBatchCapQueuesFourth(t *testing.T) {
	f := newFakes()
	f.idle = "" // no free pane: the batch is decided, the send is refused for each
	var lines []string
	for n := 21; n <= 24; n++ {
		lines = append(lines, "TASK #"+strconv.Itoa(n)+": slug=s | objective=o | files=a.go | verify=v | evidence=e")
	}
	res, err := Act(strings.Join(lines, "\n"), f.deps())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Queued) != 1 || res.Queued[0] != "TASK #24" {
		t.Errorf("queued = %v, want TASK #24", res.Queued)
	}
	if len(res.Failed) != 3 {
		t.Errorf("failed = %v, want the three started tasks refused for no idle pane", res.Failed)
	}
}

// A FOLLOWUP block creates one issue, and an ESCALATE reaches the operator.
func TestFollowupAndEscalate(t *testing.T) {
	f := newFakes()
	reply := "FOLLOWUP #22: title=split parser | done=parser | remaining=renderer | reason=size | deps=#21\nESCALATE #22: which dump?"
	res, err := Act(reply, f.deps())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.issues) != 1 || f.issues[0] != "split parser" {
		t.Errorf("issues = %v, want one issue titled split parser", f.issues)
	}
	if len(f.escalated) != 1 || !strings.Contains(f.escalated[0], "which dump?") {
		t.Errorf("escalated = %v", f.escalated)
	}
	if len(res.Dispatched) != 2 {
		t.Errorf("dispatched = %v", res.Dispatched)
	}
}

// A failing effect is kept in Failed; the rest of the reply still runs.
func TestFailureDoesNotStopOthers(t *testing.T) {
	f := newFakes()
	d := f.deps()
	d.CreateIssue = func(string, string) (int, error) { return 0, errors.New("gh: down") }
	reply := "FOLLOWUP #22: title=x | done=a | remaining=b | reason=c | deps=#1\nESCALATE #22: still runs?"
	res, err := Act(reply, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failed) != 1 || len(f.escalated) != 1 {
		t.Errorf("failed = %v, escalated = %v; want the failure kept and the escalation sent", res.Failed, f.escalated)
	}
}

// Act tells Assign which sub-issue each pane was sent.
func TestActAssignsPanes(t *testing.T) {
	f := newFakes()
	d := f.deps()
	got := map[string]int{}
	d.Assign = func(issue int, pane string) { got[pane] = issue }
	if _, err := Act("POINTS #21: A=P1\nTASK #22: slug=x | objective=y | files=a.go | verify=z | evidence=e", d); err != nil {
		t.Fatal(err)
	}
	if got["investigator-a"] != 21 || got[f.idle] != 22 {
		t.Fatalf("assigned %v", got)
	}
}
