package spawner

import (
	"fmt"
	"os/exec"
	"strings"
)

// Baseline refuses to dispatch unless Ponytail is an enabled plugin and RTK is the RTK binary.
// `rtk gain` prints "RTK Token Savings" only for RTK; a different rtk on PATH fails it.
func Baseline() error {
	var missing []string
	if out, err := exec.Command("rtk", "gain").CombinedOutput(); err != nil || !strings.Contains(string(out), "RTK Token Savings") {
		missing = append(missing, "rtk")
	}
	if out, err := exec.Command("claude", "plugin", "list").CombinedOutput(); err != nil || !pluginEnabled(string(out), "ponytail@ponytail") {
		missing = append(missing, "ponytail plugin")
	}
	if len(missing) > 0 {
		return fmt.Errorf("refusing to dispatch: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

// pluginEnabled reports whether the entry for name in `claude plugin list` says enabled.
// Each entry is a block of lines that starts with the plugin name.
func pluginEnabled(list, name string) bool {
	_, rest, found := strings.Cut(list, name)
	if !found {
		return false
	}
	block, _, _ := strings.Cut(rest, "\n\n")
	return strings.Contains(block, "enabled") && !strings.Contains(block, "disabled")
}
