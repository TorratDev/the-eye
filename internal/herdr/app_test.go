package herdr

import "testing"

func TestSpaceID(t *testing.T) {
	ls := []byte(`{"spaces":[{"id":"sp1","name":"other"},{"id":"sp2","name":"epic 7"}],"recent":[{"name":"epic 8"}]}`)
	for name, want := range map[string]string{"epic 7": "sp2", "epic 8": "", "missing": ""} {
		if got, err := spaceID(ls, name); err != nil || got != want {
			t.Errorf("spaceID(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := spaceID([]byte("not json"), "x"); err == nil {
		t.Error("unreadable ls accepted")
	}
}
