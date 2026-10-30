package prompts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The eight parts every role prompt has, in order. Each heading is "## <n>. <title>".
var parts = []string{
	"Identity", "Startup", "Required reading", "Hard rules",
	"Procedure", "Evidence standard", "Output contract", "Stop and cleanup",
}

// Text each role's output contract (part 7) must contain, so the hub's parser matches it.
var contract = map[string][]string{
	"orchestrator": {"POINTS #<n>:", "TASK #<n>:", "FOLLOWUP #<n>:", "ESCALATE #<n>:"},
	"investigator": {"ID | finding | tag | evidence | confidence | suggested task | fits one PR", "BLOCKED: <what is missing>"},
	"executor":     {"DONE: <summary>", "BLOCKED: <what is missing, failing, or too large for one PR>"},
	"reviewer":     {"PASS: <reason>", "REVISE: <one specific change>", "SPLIT: <part to move into a follow-up issue>", "ESCALATE: <why a human must decide>"},
}

var heading = regexp.MustCompile(`(?m)^## (\d+)\. (.+)$`)

func TestRolePrompts(t *testing.T) {
	for role, lines := range contract {
		t.Run(role, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "prompts", role+".md"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)

			heads := heading.FindAllStringSubmatch(text, -1)
			if len(heads) != len(parts) {
				t.Fatalf("got %d numbered parts, want %d", len(heads), len(parts))
			}
			for i, h := range heads {
				want := parts[i]
				if h[1] != string(rune('1'+i)) || !strings.HasPrefix(h[2], want) {
					t.Errorf("part %d is %q, want %q", i+1, h[0], want)
				}
			}

			// the text of part 7 runs from its heading to the next heading
			start := strings.Index(text, "## 7. ")
			end := strings.Index(text, "## 8. ")
			if start < 0 || end < start {
				t.Fatal("part 7 or 8 heading missing or out of order")
			}
			output := text[start:end]
			for _, l := range lines {
				if !strings.Contains(output, l) {
					t.Errorf("output contract (part 7) lacks %q", l)
				}
			}
		})
	}
}
