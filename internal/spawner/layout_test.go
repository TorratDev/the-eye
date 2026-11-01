package spawner

import (
	"regexp"
	"testing"
)

// hc accepts 1-32 of a-z 0-9 - _ as a terminal name.
var hcName = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

func TestPanesAreValidHCNames(t *testing.T) {
	ids := Panes(123456)
	if len(ids) != 9 || ids["executor-1"] != "e123456-executor-1" {
		t.Fatalf("Panes = %v", ids)
	}
	seen := map[string]bool{}
	for slot, id := range ids {
		if !hcName.MatchString(id) || seen[id] {
			t.Errorf("%s -> %q: not a valid, unique hc name", slot, id)
		}
		seen[id] = true
	}
}

// A session terminal joins the epic's window space only when ShowEpic found a window.
func TestLabels(t *testing.T) {
	defer func(s string) { space = s }(space)
	space = ""
	if l := labels("executor"); len(l) != 1 || l["role"] != "executor" {
		t.Errorf("no window: %v", l)
	}
	space = "sp1"
	if l := labels("reviewer"); l["herdr.space"] != "sp1" || l["role"] != "reviewer" {
		t.Errorf("with a window: %v", l)
	}
}
