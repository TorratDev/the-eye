// Package cloud reads what cloud workers post to GitHub: their reports, for the hub to apply.
package cloud

import (
	"fmt"
	"strings"

	"github.com/TorratDev/the-eye/internal/session"
)

// Marker starts every cloud worker report comment. A comment without it is not a report.
const Marker = "<!-- hub:worker-report -->"

// Report is a parsed cloud worker report.
type Report struct {
	Role  string
	Final session.Final
}

// ParseComment reads one issue comment. It is a report only if it starts with Marker; a comment
// without the marker returns ok=false and no error. A marked comment must end in a final line the
// role's output contract defines, or it is an error.
func ParseComment(body, role string) (Report, bool, error) {
	text := strings.TrimSpace(body)
	if !strings.HasPrefix(text, Marker) {
		return Report{}, false, nil
	}
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(text, Marker)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	f, err := session.ParseFinal(role, last)
	if err != nil {
		return Report{}, true, fmt.Errorf("cloud report: %w", err)
	}
	return Report{Role: role, Final: f}, true, nil
}
