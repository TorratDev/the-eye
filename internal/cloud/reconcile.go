package cloud

import "fmt"

// ClaimStore is the part of the hub store reconciliation needs: the local claims and the state setter.
type ClaimStore interface {
	StateSetter
	Claims() (map[int]string, error)
}

// Outcome says what Reconcile did with one sub-issue's comments.
type Outcome struct {
	Applied int      // reports whose state was set
	Refused []string // reports not applied because a local session holds the claim
	Routed  []string // BLOCKED reports for the orchestrator, as events
}

// Reconcile applies a sub-issue's cloud reports with the local claim in view.
//
// A sub-issue claimed locally is not started by a cloud worker: a cloud executor report on it is
// refused. A cloud BLOCKED report is always routed to the orchestrator as an event, the same as a
// local one. On a locally claimed sub-issue it does not change the state, because the local session
// owns that state.
func Reconcile(st ClaimStore, issue int, comments []Comment) (Outcome, error) {
	claims, err := st.Claims()
	if err != nil {
		return Outcome{}, err
	}
	locked := claims[issue] != ""
	var out Outcome
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
		if r.Final.Kind == "BLOCKED" {
			out.Routed = append(out.Routed, fmt.Sprintf("#%d %s blocked: %s", issue, r.Role, r.Final.Text))
			if locked {
				continue
			}
		}
		if locked && r.Role == "executor" {
			out.Refused = append(out.Refused, fmt.Sprintf("#%d: cloud %s report refused, claimed locally", issue, r.Role))
			continue
		}
		state, changes := StateFor(r)
		if !changes {
			continue
		}
		if err := st.SetState(issue, state); err != nil {
			return out, err
		}
		out.Applied++
	}
	return out, first
}
