package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/ui"
)

// A cloud BLOCKED finding is stored as an alert and shows in the page the live stream carries.
func TestCloudBlockedShowsOnPage(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	recordAlert(st, live.NewBroker(), 23, "cloud BLOCKED", "missing dump")

	frag, err := ui.SessionsFragment(st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frag, "#23 cloud BLOCKED: missing dump") {
		t.Errorf("errors panel lacks the cloud BLOCKED finding:\n%s", frag)
	}
}
