package spawner

import (
	"strings"
	"testing"
)

// With RTK off PATH the launcher must refuse and name the missing tool.
func TestBaselineRefusesWithoutRTK(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := Baseline()
	if err == nil || !strings.Contains(err.Error(), "rtk") {
		t.Fatalf("Baseline() = %v, want refusal naming rtk", err)
	}
}

func TestPluginEnabled(t *testing.T) {
	list := "  ❯ gopls-lsp@claude-plugins-official\n    Status: ✔ enabled\n\n  ❯ ponytail@ponytail\n    Status: ✘ disabled\n"
	if pluginEnabled(list, "ponytail@ponytail") {
		t.Error("disabled ponytail reported enabled")
	}
	list = strings.Replace(list, "disabled", "enabled", 1)
	if !pluginEnabled(list, "ponytail@ponytail") {
		t.Error("enabled ponytail reported disabled")
	}
	if pluginEnabled(list, "missing@x") {
		t.Error("unknown plugin reported enabled")
	}
}
