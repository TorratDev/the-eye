package spawner

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Dispatch fields the roles' prompts say they receive. Each builder checks what the prompt
// relies on, so a session never starts from a message with a field missing.

// InvestigatorDispatch is the message to one investigator (prompts/investigator.md, part 2).
type InvestigatorDispatch struct {
	Letter    string // A, B or C
	Issue     int
	Points    []string // P1, P2, ...
	Resources string   // optional, for example a tool project copy
}

// ExecutorDispatch is the message to one executor (prompts/executor.md, part 2).
type ExecutorDispatch struct {
	Issue     int
	Worktree  string
	Branch    string // <n>-<short-slug>
	Objective string
	Files     []string
	Verify    string
	Evidence  string
}

// ReviewDispatch is the message to one reviewer (prompts/reviewer.md, part 2).
type ReviewDispatch struct {
	Issue     int
	Objective string
	Files     []string
	Verify    string
	Evidence  string
	Done      string // the executor's DONE or BLOCKED line
	Worktree  string
	Branch    string
	Resources string // optional
}

var branchRE = regexp.MustCompile(`^\d+-[a-z0-9-]+$`)

func (d InvestigatorDispatch) Message() (string, error) {
	if !slices.Contains([]string{"A", "B", "C"}, d.Letter) {
		return "", fmt.Errorf("investigator letter %q is not A, B or C", d.Letter)
	}
	if d.Issue <= 0 || len(d.Points) == 0 {
		return "", fmt.Errorf("investigator dispatch needs an issue and at least one point")
	}
	return lines(
		"role: investigator",
		"letter: "+d.Letter,
		fmt.Sprintf("issue: #%d", d.Issue),
		"points: "+strings.Join(d.Points, ", "),
		optional("resources", d.Resources),
	), nil
}

func (d ExecutorDispatch) Message() (string, error) {
	if d.Issue <= 0 || d.Worktree == "" || d.Objective == "" || d.Verify == "" || d.Evidence == "" || len(d.Files) == 0 {
		return "", fmt.Errorf("executor dispatch needs an issue, worktree, objective, files, verify and evidence")
	}
	if !branchRE.MatchString(d.Branch) || !strings.HasPrefix(d.Branch, fmt.Sprint(d.Issue)+"-") {
		return "", fmt.Errorf("branch %q is not <n>-<short-slug> for issue #%d", d.Branch, d.Issue)
	}
	return lines(
		"role: executor",
		fmt.Sprintf("issue: #%d", d.Issue),
		"worktree: "+d.Worktree,
		"branch: "+d.Branch,
		"objective: "+d.Objective,
		"files: "+strings.Join(d.Files, ", "),
		"verify: "+d.Verify,
		"evidence: "+d.Evidence,
	), nil
}

func (d ReviewDispatch) Message() (string, error) {
	if d.Issue <= 0 || d.Objective == "" || d.Verify == "" || d.Done == "" || d.Worktree == "" || d.Branch == "" || len(d.Files) == 0 {
		return "", fmt.Errorf("review dispatch needs an issue, objective, files, verify, the executor line, worktree and branch")
	}
	return lines(
		"role: reviewer",
		fmt.Sprintf("issue: #%d", d.Issue),
		"objective: "+d.Objective,
		"files: "+strings.Join(d.Files, ", "),
		"verify: "+d.Verify,
		"evidence: "+d.Evidence,
		"executor: "+d.Done,
		"worktree: "+d.Worktree,
		"branch: "+d.Branch,
		optional("resources", d.Resources),
	), nil
}

func lines(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}

func optional(key, value string) string {
	if value == "" {
		return ""
	}
	return key + ": " + value
}
