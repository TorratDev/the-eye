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
