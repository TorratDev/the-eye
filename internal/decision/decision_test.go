package decision

import (
	"strings"
	"testing"
)

// A TASK block becomes a dispatched executor task whose message the executor prompt accepts.
func TestTaskBecomesExecutorDispatch(t *testing.T) {
	reply := "Plan done.\nTASK #22: slug=dispatch | objective=build the messages | files=internal/spawner/message.go,internal/spawner/message_test.go | verify=go test ./... | evidence=test output\n"
	acts, err := Route(reply, func(branch string) string { return "/wt/" + branch })
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].Kind != "execute" {
		t.Fatalf("actions = %+v, want one execute", acts)
	}
	d := acts[0].Execute
	if d.Branch != "22-dispatch" || d.Worktree != "/wt/22-dispatch" || len(d.Files) != 2 {
		t.Errorf("dispatch = %+v", d)
	}
	msg, err := d.Message()
	if err != nil || !strings.Contains(msg, "branch: 22-dispatch") {
		t.Errorf("message = %q, %v", msg, err)
	}
}

func TestRouteAllKinds(t *testing.T) {
	reply := strings.Join([]string{
		"POINTS #22: A=P1,P2 B=P3",
		"FOLLOWUP #22: title=split parser | done=parser | remaining=renderer | reason=size | deps=#21",
		"ESCALATE #22: which dump?",
		"some prose the hub ignores",
	}, "\n")
	acts, err := Route(reply, func(string) string { return "/wt" })
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	for _, a := range acts {
		kinds = append(kinds, a.Kind)
	}
	if strings.Join(kinds, ",") != "investigate,followup,escalate" {
		t.Errorf("kinds = %v", kinds)
	}
	if got := acts[0].Investigate; len(got) != 2 || got[0].Letter != "A" || len(got[0].Points) != 2 {
		t.Errorf("investigators = %+v", got)
	}
	if acts[1].Followup.Fields["title"] != "split parser" {
		t.Errorf("followup = %+v", acts[1].Followup)
	}
}

func TestBadTaskRefused(t *testing.T) {
	_, err := Route("TASK #22: slug=x | objective= | files= | verify= | evidence=", func(string) string { return "/wt" })
	if err == nil {
		t.Error("TASK with no objective accepted")
	}
}

// liveReply is the orchestrator's reply from the first live run (#99), as the Stop hook's
// last_assistant_message carried it: prose, then the decision line inside a code fence.
const liveReply = "Epic #117 is a throwaway smoke test for the #99 live run. Its only sub-issue is #118, which lists no tasks and says to plan nothing. So there is nothing to investigate, split, or dispatch.\n\nI reported #118 as done, with the one-line summary that it has no points or task. I did not comment on either issue or touch the repo.\n\nDecision for the hub:\n\n```\nPOINTS #118: A= B= C=\n```\n\nNo `TASK`, `FOLLOWUP`, or `ESCALATE` lines. I'm stopping here since the epic is complete once the hub closes #118 by hand after the run."

// "A= B= C=" starts no investigators.
func TestLiveReplyDispatchesNothing(t *testing.T) {
	acts, err := Route(liveReply, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 0 {
		t.Fatalf("live reply dispatched %+v", acts)
	}
}

// Evidence that needs a PR or a push can't exist before review, since the hub opens the PR after
// PASS (#169): such a TASK is refused before dispatch.
func TestPREvidenceRefused(t *testing.T) {
	wt := func(string) string { return "/wt" }
	base := "TASK #22: slug=x | objective=add a line | files=docs/a.md | verify=go test ./... | evidence="
	for evidence, refused := range map[string]bool{
		"tail -n 1 docs/a.md shows the line; verify output":          false,
		"verify output; PR body contains Closes #22":                 true,
		"the pull request links #22":                                 true,
		"branch pushed to origin":                                    true,
		"git diff shows one added line; reproduces the prior output": false,
	} {
		_, err := Route(base+evidence, wt)
		if (err != nil) != refused {
			t.Errorf("evidence %q: err = %v, want refused %v", evidence, err, refused)
		}
	}
}
