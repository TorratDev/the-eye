// Package decision reads the orchestrator's decision lines and turns each into the hub action it asks for.
package decision

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/TorratDev/the-eye/internal/spawner"
)

// Action is one routed decision. Exactly one of the fields is set, by Kind.
type Action struct {
	Kind        string // investigate, execute, followup, escalate
	Investigate []spawner.InvestigatorDispatch
	Execute     *spawner.ExecutorDispatch
	Followup    *Followup
	Escalate    string
}

// Followup is a follow-up issue the orchestrator asks for; the hub creates it.
type Followup struct {
	Issue  int
	Fields map[string]string // title, done, remaining, reason, deps
}

var lineRE = regexp.MustCompile(`^(POINTS|TASK|FOLLOWUP|ESCALATE) #(\d+): (.+)$`)

// Route parses the orchestrator's decision lines and routes each one. worktree gives the
// executor's worktree path for a branch name, so a TASK can be dispatched. Unknown lines are an error.
func Route(reply string, worktree func(branch string) string) ([]Action, error) {
	var out []Action
	for _, raw := range strings.Split(reply, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		m := lineRE.FindStringSubmatch(line)
		if m == nil {
			continue // prose around the blocks is not a decision
		}
		n, _ := strconv.Atoi(m[2])
		body := m[3]
		switch m[1] {
		case "POINTS":
			inv, err := points(n, body)
			if err != nil {
				return nil, err
			}
			if len(inv) == 0 {
				continue
			}
			out = append(out, Action{Kind: "investigate", Investigate: inv})
		case "TASK":
			ex, err := task(n, body, worktree)
			if err != nil {
				return nil, err
			}
			out = append(out, Action{Kind: "execute", Execute: ex})
		case "FOLLOWUP":
			out = append(out, Action{Kind: "followup", Followup: &Followup{Issue: n, Fields: fields(body)}})
		case "ESCALATE":
			out = append(out, Action{Kind: "escalate", Escalate: fmt.Sprintf("#%d: %s", n, body)})
		}
	}
	return out, nil
}

// points reads "A=P1,P2 B=P3 C=P4" into one investigator dispatch per letter. A letter with no
// points ("A=") gets no dispatch, so "A= B= C=" means nothing to investigate.
func points(issue int, body string) ([]spawner.InvestigatorDispatch, error) {
	var out []spawner.InvestigatorDispatch
	for _, part := range strings.Fields(body) {
		letter, list, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("POINTS #%d: %q is not LETTER=points", issue, part)
		}
		if list == "" {
			continue
		}
		out = append(out, spawner.InvestigatorDispatch{Letter: letter, Issue: issue, Points: strings.Split(list, ",")})
	}
	return out, nil
}

// task reads "slug=... | objective=... | files=a,b | verify=... | evidence=..." into an executor dispatch.
func task(issue int, body string, worktree func(string) string) (*spawner.ExecutorDispatch, error) {
	f := fields(body)
	branch := fmt.Sprintf("%d-%s", issue, f["slug"])
	d := &spawner.ExecutorDispatch{
		Issue:     issue,
		Worktree:  worktree(branch),
		Branch:    branch,
		Objective: f["objective"],
		Files:     strings.Split(f["files"], ","),
		Verify:    f["verify"],
		Evidence:  f["evidence"],
	}
	if _, err := d.Message(); err != nil {
		return nil, fmt.Errorf("TASK #%d: %w", issue, err)
	}
	if prEvidence.MatchString(d.Evidence) {
		return nil, fmt.Errorf("TASK #%d: evidence %q needs a PR or a push, but the hub opens the PR only after the reviewer's PASS: ask for evidence the worktree shows (#169)", issue, d.Evidence)
	}
	return d, nil
}

// prEvidence matches evidence that only exists once a branch is pushed or a PR is open.
var prEvidence = regexp.MustCompile(`(?i)\bPRs?\b|\bpull request|\bpush(ed|es)?\b`)

// fields splits "key=value | key=value" into a map; a value keeps any "=" inside it.
func fields(body string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(body, "|") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}
