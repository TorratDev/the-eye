package session

import "testing"

func TestParseFinal(t *testing.T) {
	ok := []struct{ role, line, kind string }{
		{"executor", "DONE: tests pass; verification: ok; PR: none", "DONE"},
		{"executor", "BLOCKED: rebase conflict in internal/x.go", "BLOCKED"},
		{"investigator", "BLOCKED: missing dump", "BLOCKED"},
		{"investigator", "1 | finding | [verified] | file:3 | high | task | yes", "FINDING"},
		{"reviewer", "PASS: diff meets the objective", "PASS"},
		{"reviewer", "REVISE: rename the helper", "REVISE"},
		{"reviewer", "SPLIT: move the parser to a follow-up", "SPLIT"},
		{"reviewer", "ESCALATE: legal check failed", "ESCALATE"},
		{"orchestrator", "POINTS #22: A=P1 B=P2", "POINTS"},
		{"orchestrator", "TASK #22: slug=x | objective=y | files=a | verify=z | evidence=e", "TASK"},
		{"orchestrator", "FOLLOWUP #22: title=t | done=d | remaining=r | reason=x | deps=#1", "FOLLOWUP"},
		{"orchestrator", "ESCALATE #22: which dump?", "ESCALATE"},
	}
	for _, c := range ok {
		f, err := ParseFinal(c.role, c.line)
		if err != nil || f.Kind != c.kind {
			t.Errorf("ParseFinal(%s, %q) = %+v, %v; want kind %s", c.role, c.line, f, err, c.kind)
		}
	}
	bad := []struct{ role, line string }{
		{"executor", "PASS: looks fine"},          // reviewer verdict from an executor
		{"reviewer", "ESCALATE #22: which dump?"}, // orchestrator form on a reviewer
		{"orchestrator", "ESCALATE: no issue number"},
		{"executor", "I finished the work."},
		{"reviewer", ""},
		{"nobody", "PASS: x"},
	}
	for _, c := range bad {
		if _, err := ParseFinal(c.role, c.line); err == nil {
			t.Errorf("ParseFinal(%s, %q) accepted, want error", c.role, c.line)
		}
	}
}
