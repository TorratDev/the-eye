package cloud

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

// Comment is one issue comment as gh reports it.
type Comment struct {
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

// StateSetter is the part of the hub store the ingest needs.
type StateSetter interface {
	SetState(number int, state string) error
}

// FetchComments returns every comment on an issue, oldest first, through the gh login.
func FetchComments(repo string, issue int) ([]Comment, error) {
	out, err := exec.Command("gh", "issue", "view", strconv.Itoa(issue), "--repo", repo, "--json", "comments").Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue view #%d: %w", issue, err)
	}
	var v struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("gh issue view #%d: %w", issue, err)
	}
	return v.Comments, nil
}

// StateFor maps a report to the sub-issue state the local sessions use. A finding carries no
// state change.
func StateFor(r Report) (string, bool) {
	switch r.Final.Kind {
	case "DONE":
		return "in review", true
	case "BLOCKED":
		return "blocked", true
	case "PASS":
		return "approved", true
	case "REVISE":
		return "revise", true
	case "SPLIT", "ESCALATE":
		return "escalated", true
	}
	return "", false
}

// Apply reads the comments of one sub-issue in order and sets the state for each report. The last
// report wins. It returns the number of reports applied and an error for the first marked comment
// that is not a valid report; the valid ones are still applied.
func Apply(st StateSetter, issue int, comments []Comment) (int, error) {
	applied := 0
	var first error
	for _, c := range comments {
		r, ok, err := ParseComment(c.Body)
		if !ok {
			continue
		}
		if err != nil {
			if first == nil {
				first = fmt.Errorf("#%d: %w", issue, err)
			}
			continue
		}
		state, changes := StateFor(r)
		if !changes {
			continue
		}
		if err := st.SetState(issue, state); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, first
}
