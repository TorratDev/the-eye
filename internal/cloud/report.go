// Package cloud reads what cloud workers post to GitHub: their reports, for the hub to apply.
package cloud

import (
	"fmt"
	"strings"

	"github.com/TorratDev/the-eye/internal/session"
)

// Marker starts every cloud worker report comment. A comment without it is not a report.
const Marker = "<!-- hub:worker-report -->"

// roleLine names the worker's role on the line right after the marker, e.g. "role: executor".
const roleLine = "role: "

// Report is a parsed cloud worker report.
type Report struct {
	Role  string
	Final session.Final
}

// ParseComment reads one issue comment. It is a report only if it starts with Marker, followed by a
// "role: <name>" line; any other comment returns ok=false and no error. A report must end in a final
// line its role's output contract defines, or it is an error.
func ParseComment(body string) (Report, bool, error) {
	text := strings.TrimSpace(body)
	if !strings.HasPrefix(text, Marker) {
		return Report{}, false, nil
	}
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(text, Marker)), "\n")
	first := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(first, roleLine) {
		return Report{}, true, fmt.Errorf("cloud report: no %q line after the marker", strings.TrimSpace(roleLine))
	}
	role := strings.TrimSpace(strings.TrimPrefix(first, roleLine))
	last := strings.TrimSpace(lines[len(lines)-1])
	f, err := session.ParseFinal(role, last)
	if err != nil {
		return Report{}, true, fmt.Errorf("cloud report: %w", err)
	}
	return Report{Role: role, Final: f}, true, nil
}
