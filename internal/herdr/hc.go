package herdr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// HCPinnedVersion is the herdr-core release the hc wrappers are tested against. Move it only with
// the test. herdr-core (`hc`) runs the terminals the herdr desktop app shows (#151).
const HCPinnedVersion = "0.1.16"

// HCBin is the herdr-core executable.
var HCBin = "hc"

// Terminal holds the fields the hub reads from an hc terminal.
type Terminal struct {
	Address string            `json:"address"` // t7: this terminal for the life of the core
	Name    string            `json:"name"`
	State   string            `json:"state"` // live or finished
	Labels  map[string]string `json:"labels"`
	Status  Status            `json:"status"` // zero until the agent reports
}

// Status is what the agent in a terminal reports about itself. State is idle, working, done,
// blocked or error; for blocked, Kind is permission, question or auth, and Msg says what it waits on.
type Status struct {
	State string `json:"state"`
	Kind  string `json:"kind"`
	Msg   string `json:"msg"`
}

// HCVersion returns the installed hc version, e.g. "0.1.16".
func HCVersion() (string, error) {
	out, err := exec.Command(HCBin, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "hc "), nil
}

// hc runs one hc command with JSON output and returns its result.
func hc(cmd string, args ...string) (json.RawMessage, error) {
	out, err := exec.Command(HCBin, append([]string{cmd, "--json"}, args...)...).Output()
	// hc writes its error object to stderr
	if ee, ok := err.(*exec.ExitError); ok && len(out) == 0 {
		out = ee.Stderr
	}
	return hcDecode(cmd, out, err)
}

// hcDecode reads one command's output: the result object, or the error object hc prints when it
// refuses or fails.
func hcDecode(cmd string, out []byte, err error) (json.RawMessage, error) {
	var env struct {
		Error *struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if jerr := json.Unmarshal(out, &env); jerr != nil {
		return nil, fmt.Errorf("hc %s: unreadable output (%v): %s", cmd, err, strings.TrimSpace(string(out)))
	}
	if env.Error != nil {
		return nil, fmt.Errorf("hc %s: %s: %s", cmd, env.Error.Kind, env.Error.Message)
	}
	if err != nil {
		return nil, fmt.Errorf("hc %s: %w", cmd, err)
	}
	return out, nil
}

// Run starts line in a shell in a new terminal named name, in cwd ("" for hc's own), and returns
// at once. It has no parent: when the hub itself runs in an hc terminal, its sessions must not
// end with it or type notes into it.
func Run(name, cwd, line string, labels map[string]string) (Terminal, error) {
	args := []string{"--bg", "--no-parent", "--name", name}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--label", k+"="+labels[k])
	}
	res, err := hc("run", append(args, "--", "sh", "-c", line)...)
	if err != nil {
		return Terminal{}, err
	}
	var t Terminal
	return t, json.Unmarshal(res, &t)
}

// Get returns one terminal, including its agent's status.
func Get(terminal string) (Terminal, error) {
	res, err := hc("get", terminal)
	if err != nil {
		return Terminal{}, err
	}
	var t Terminal
	return t, json.Unmarshal(res, &t)
}

// Read returns a terminal's screen and lines of history above it. It works before the agent has
// reported, unlike `herdr agent read`.
func Read(terminal string, lines int) (string, error) {
	res, err := hc("read", "--lines", strconv.Itoa(lines), terminal)
	if err != nil {
		return "", err
	}
	var v struct {
		Text string `json:"text"`
	}
	return v.Text, json.Unmarshal(res, &v)
}

// Keys presses named keys in a terminal, e.g. "enter" or "esc". hc holds them until nobody has
// typed for about three seconds.
func Keys(terminal string, keys ...string) error {
	_, err := hc("keys", append([]string{terminal}, keys...)...)
	return err
}

// Send pastes text into a terminal and presses Enter. It reaches an agent mid-turn. hc refuses
// while the agent is blocked, since Enter would answer its dialog; the error then names kind
// "blocked".
func Send(terminal, text string) error {
	_, err := hc("send", terminal, text)
	return err
}

// Wait blocks until the terminal's agent stops working (any state but working, or the program
// ended), or timeout passes. It returns the state it saw and whether the wait ended; a timeout
// returns ended false with the current state.
func Wait(terminal string, timeout time.Duration) (state string, ended bool, err error) {
	secs := max(1, int(timeout/time.Second))
	res, err := hc("wait", "--timeout", strconv.Itoa(secs), terminal)
	if err != nil {
		return "", false, err
	}
	var v struct {
		Ended  bool    `json:"ended"`
		Status *string `json:"status"`
	}
	if err := json.Unmarshal(res, &v); err != nil {
		return "", false, err
	}
	if v.Status != nil {
		state = *v.Status
	}
	return state, v.Ended, nil
}
