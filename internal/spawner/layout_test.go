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
