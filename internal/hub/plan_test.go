package hub

import (
	"slices"
	"testing"
)

func open(n int, blockers ...Ref) Issue {
	return Issue{Number: n, State: "OPEN", BlockedBy: blockers}
}

func readyNumbers(vs []Verdict) []int {
	var out []int
	for _, v := range vs {
		if v.Ready {
			out = append(out, v.Issue.Number)
		}
	}
	return out
}

// A blocks B blocks C: B and C wait, so only A is ready.
func TestPlanChainOnlyFirstIsReady(t *testing.T) {
	issues := []Issue{
		open(1),
		open(2, Ref{Number: 1, State: "OPEN"}),
		open(3, Ref{Number: 2, State: "OPEN"}),
	}
	verdicts, cycles := Plan(issues, nil)
	if len(cycles) != 0 {
		t.Fatalf("cycles = %v, want none", cycles)
	}
	if got := readyNumbers(verdicts); !slices.Equal(got, []int{1}) {
		t.Fatalf("ready = %v, want [1]", got)
	}
}

func TestPlanCycleReportedAndNotDispatched(t *testing.T) {
	issues := []Issue{
		open(1, Ref{Number: 2, State: "OPEN"}), // 1 <-> 2
		open(2, Ref{Number: 1, State: "OPEN"}),
		open(3, Ref{Number: 1, State: "OPEN"}), // downstream of the cycle
		open(4),                                // unrelated
	}
	verdicts, cycles := Plan(issues, nil)
	if len(cycles) != 1 || !slices.Contains(cycles[0], 1) || !slices.Contains(cycles[0], 2) {
		t.Fatalf("cycles = %v, want one cycle over 1 and 2", cycles)
	}
	if got := readyNumbers(verdicts); !slices.Equal(got, []int{4}) {
		t.Fatalf("ready = %v, want [4]", got)
	}
}

func TestPlanSelfBlockIsCycle(t *testing.T) {
	_, cycles := Plan([]Issue{open(7, Ref{Number: 7, State: "OPEN"})}, nil)
	if len(cycles) != 1 || !slices.Equal(cycles[0], []int{7}) {
		t.Fatalf("cycles = %v, want [[7]]", cycles)
	}
}

func TestPlanBlockersOutsideEpic(t *testing.T) {
	issues := []Issue{
		open(1, Ref{Number: 99, State: "OPEN"}),   // open outside blocker holds 1
		open(2, Ref{Number: 98, State: "CLOSED"}), // closed outside blocker frees 2
	}
	verdicts, _ := Plan(issues, nil)
	if got := readyNumbers(verdicts); !slices.Equal(got, []int{2}) {
		t.Fatalf("ready = %v, want [2]", got)
	}
	if !slices.Equal(verdicts[0].Waiting, []int{99}) {
		t.Fatalf("waiting = %v, want [99]", verdicts[0].Waiting)
	}
}

func TestPlanClaimedAndClosedAreNotReady(t *testing.T) {
	issues := []Issue{
		open(1),
		open(2),
		{Number: 3, State: "CLOSED"},
		{Number: 4, State: "OPEN", Labels: []string{ClaimLabel}},
	}
	verdicts, _ := Plan(issues, map[int]string{1: "run-a"})
	if got := readyNumbers(verdicts); !slices.Equal(got, []int{2}) {
		t.Fatalf("ready = %v, want [2]", got)
	}
}
