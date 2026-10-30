// Package session reads what a role session reports: its final line and its state.
package session

import (
	"fmt"
	"regexp"
	"strings"
)

// Final is the parsed last line of a role's reply.
type Final struct {
	Kind string // DONE, BLOCKED, PASS, REVISE, SPLIT, ESCALATE, POINTS, TASK, FOLLOWUP, FINDING
	Text string // the rest of the line after the kind
}

var finals = map[string]*regexp.Regexp{
	"executor":     regexp.MustCompile(`^(DONE|BLOCKED): (.+)$`),
	"investigator": regexp.MustCompile(`^(BLOCKED): (.+)$`),
	"reviewer":     regexp.MustCompile(`^(PASS|REVISE|SPLIT|ESCALATE): (.+)$`),
	"orchestrator": regexp.MustCompile(`^(POINTS|TASK|FOLLOWUP|ESCALATE) #\d+: (.+)$`),
}

// ParseFinal parses the last line of role's reply. It accepts only the formats in the role's
// output contract. Anything else is an error, never a guess.
func ParseFinal(role, line string) (Final, error) {
	line = strings.TrimSpace(line)
	re, ok := finals[role]
	if !ok {
		return Final{}, fmt.Errorf("no final-line format for role %q", role)
	}
	if m := re.FindStringSubmatch(line); m != nil {
		return Final{Kind: m[1], Text: m[2]}, nil
	}
	// investigators end with a findings table row: ID | finding | tag | evidence | confidence | suggested task | fits one PR
	if role == "investigator" && strings.Count(line, "|") == 6 {
		return Final{Kind: "FINDING", Text: line}, nil
	}
	return Final{}, fmt.Errorf("unparsed final line for %s: %q", role, line)
}
