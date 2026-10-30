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

// findCycles finds the strongly connected components of the blocked-by graph
// between open sub-issues (Tarjan). A component of more than one issue, or an
// issue that blocks itself, is a cycle. Blockers outside the epic and closed
// issues are skipped: they cannot be part of a cycle. Members are sorted.
func findCycles(issues []Issue, byNum map[int]Issue) [][]int {
	inGraph := func(n int) bool {
		i, ok := byNum[n]
		return ok && i.State != "CLOSED"
	}
	index := map[int]int{}
	low := map[int]int{}
	onStack := map[int]bool{}
	var stack []int
	var cycles [][]int
	var visit func(n int)
	visit = func(n int) {
		index[n] = len(index)
		low[n] = index[n]
		stack = append(stack, n)
		onStack[n] = true
		self := false
		for _, b := range byNum[n].BlockedBy {
			switch {
			case !inGraph(b.Number):
				continue
			case b.Number == n:
				self = true
			case !hasIndex(index, b.Number):
				visit(b.Number)
				low[n] = min(low[n], low[b.Number])
			case onStack[b.Number]:
				low[n] = min(low[n], index[b.Number])
			}
		}
		if low[n] != index[n] {
			return
		}
		at := slices.Index(stack, n)
		scc := slices.Clone(stack[at:])
		stack = stack[:at]
		for _, m := range scc {
			onStack[m] = false
		}
		if len(scc) > 1 || self {
			slices.Sort(scc)
			cycles = append(cycles, scc)
		}
	}
	for _, i := range issues {
		if inGraph(i.Number) && !hasIndex(index, i.Number) {
			visit(i.Number)
		}
	}
	return cycles
}

func hasIndex(index map[int]int, n int) bool {
	_, ok := index[n]
	return ok
}
