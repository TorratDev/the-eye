package hub

import "slices"

// Verdict is one sub-issue with its dispatch readiness.
type Verdict struct {
	Issue   Issue
	Claimed bool
	Cycle   bool
	Ready   bool
	Waiting []int // blockers that are not closed
}

// Plan computes the ready set. claims maps issue number to the local claim owner.
// A sub-issue is ready when it is open, unclaimed, not in a cycle, and every blocker is closed.
// It also returns every dependency cycle; nothing on a cycle is ready.
func Plan(issues []Issue, claims map[int]string) ([]Verdict, [][]int) {
	byNum := make(map[int]Issue, len(issues))
	for _, i := range issues {
		byNum[i.Number] = i
	}
	cycles := findCycles(issues, byNum)
	inCycle := map[int]bool{}
	for _, c := range cycles {
		for _, n := range c {
			inCycle[n] = true
		}
	}

	verdicts := make([]Verdict, 0, len(issues))
	for _, i := range issues {
		v := Verdict{
			Issue:   i,
			Claimed: claims[i.Number] != "" || slices.Contains(i.Labels, ClaimLabel),
			Cycle:   inCycle[i.Number],
		}
		for _, b := range i.BlockedBy {
			if b.State != "CLOSED" {
				v.Waiting = append(v.Waiting, b.Number)
			}
		}
		v.Ready = i.State == "OPEN" && !v.Claimed && !v.Cycle && len(v.Waiting) == 0
		verdicts = append(verdicts, v)
	}
	return verdicts, cycles
}

// findCycles runs a depth-first search over blocked-by edges between sub-issues.
// Blockers outside the epic are leaves.
func findCycles(issues []Issue, byNum map[int]Issue) [][]int {
	const (
		unvisited = iota
		onStack
		done
	)
	state := map[int]int{}
	var cycles [][]int
	var stack []int
	var visit func(n int)
	visit = func(n int) {
		state[n] = onStack
		stack = append(stack, n)
		for _, b := range byNum[n].BlockedBy {
			if _, inEpic := byNum[b.Number]; !inEpic {
				continue
			}
			switch state[b.Number] {
			case unvisited:
				visit(b.Number)
			case onStack:
				cycles = append(cycles, slices.Clone(stack[slices.Index(stack, b.Number):]))
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
	}
	for _, i := range issues {
		if state[i.Number] == unvisited {
			visit(i.Number)
		}
	}
	return cycles
}
