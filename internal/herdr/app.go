package herdr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// AppBin is the herdr desktop app's CLI. It only changes what the window shows; hc runs the
// terminals either way.
var AppBin = "herdr-app"

// AppSpace returns the window's space named name, making it (in folder, with a first tab named
// "shell") when there is none. It returns "" and no error when no window is open.
func AppSpace(name, folder string) (string, error) {
	out, err := exec.Command(AppBin, "ls").Output()
	if ee, ok := err.(*exec.ExitError); ok && strings.Contains(string(ee.Stderr), "no Herdr window") {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("herdr-app ls: %w", err)
	}
	id, err := spaceID(out, name)
	if err != nil || id != "" {
		return id, err
	}
	// space.new always opens a first tab with a shell; it stays, named so it isn't mistaken for a session
	out, err = exec.Command(AppBin, "do", "space.new", "--name", name, "--folder", folder, "--tab", "shell").Output()
	if err != nil {
		return "", fmt.Errorf("herdr-app space.new: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var v struct {
		Space string `json:"space"`
	}
	if err := json.Unmarshal(out, &v); err != nil || v.Space == "" {
		return "", fmt.Errorf("herdr-app space.new: unreadable output: %s", strings.TrimSpace(string(out)))
	}
	return v.Space, nil
}

// spaceID finds the space named name in `herdr-app ls` output; "" when there is none.
func spaceID(ls []byte, name string) (string, error) {
	var v struct {
		Spaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"spaces"`
	}
	if err := json.Unmarshal(ls, &v); err != nil {
		return "", fmt.Errorf("herdr-app ls: unreadable output: %w", err)
	}
	for _, s := range v.Spaces {
		if s.Name == name {
			return s.ID, nil
		}
	}
	return "", nil
}

// SpaceLabel is the hc label that puts a terminal in a window space; the window gives it a tab.
const SpaceLabel = "herdr.space"

// Label sets one label on a terminal.
func Label(terminal, key, value string) error {
	_, err := call("label", terminal, key+"="+value)
	return err
}
