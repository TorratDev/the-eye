package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/TorratDev/the-eye/internal/hub"
)

const (
	usage     = "usage: hub sync [--repo owner/name] <epic>"
	stateFile = ".hub/state.db" // gitignored
	graphQL   = "https://api.github.com/graphql"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hub:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "sync" {
		return errors.New(usage)
	}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	repo := fs.String("repo", "", "owner/name (default: origin remote)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(usage)
	}
	epic, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("epic must be an issue number: %w", err)
	}
	if *repo == "" {
		if *repo, err = originRepo(); err != nil {
			return err
		}
	}
	owner, name, err := parseRepo(*repo)
	if err != nil {
		return err
	}
	return sync(owner, name, epic)
}

// sync prints every sub-issue with its state, blockers and ready flag. It dispatches nothing.
func sync(owner, name string, epic int) error {
	token, err := hub.GhToken()
	if err != nil {
		return err
	}
	issues, err := (&hub.Client{URL: graphQL, Token: token}).Epic(owner, name, epic)
	if err != nil {
		return err
	}
	st, err := hub.OpenStore(stateFile)
	if err != nil {
		return err
	}
	defer st.Close()
	claims, err := st.Claims()
	if err != nil {
		return err
	}
	verdicts, cycles := hub.Plan(issues, claims)

	ready := 0
	for _, v := range verdicts {
		if v.Ready {
			ready++
		}
	}
	fmt.Printf("epic #%d in %s/%s: %d sub-issues, %d ready\n", epic, owner, name, len(issues), ready)
	for _, v := range verdicts {
		fmt.Printf("#%-4d %-6s ready=%-5t %s %s\n", v.Issue.Number, v.Issue.State, v.Ready, detail(v, claims), v.Issue.Title)
	}
	for _, c := range cycles {
		fmt.Fprintf(os.Stderr, "cycle: %s\n", cycleText(c))
	}
	if len(cycles) > 0 {
		return fmt.Errorf("%d dependency cycle(s); nothing on them is ready", len(cycles))
	}
	return nil
}

// detail says why a sub-issue is or is not ready.
func detail(v hub.Verdict, claims map[int]string) string {
	switch {
	case v.Issue.State != "OPEN":
		return "closed"
	case v.Cycle:
		return "in cycle"
	case v.Claimed && claims[v.Issue.Number] != "":
		return "claimed by " + claims[v.Issue.Number]
	case v.Claimed:
		return "claimed on GitHub"
	case len(v.Waiting) > 0:
		return "waiting on " + refList(v.Waiting)
	default:
		return "-"
	}
}

func refList(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("#%d", n)
	}
	return strings.Join(parts, ", ")
}

func cycleText(nums []int) string {
	return refList(append(nums, nums[0])) // close the loop: #a, #b, #a
}

// originRepo reads owner/name from the origin remote of the current directory.
func originRepo() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("no --repo given and no git origin remote: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parseRepo accepts owner/name, an https URL or an ssh remote, with or without .git.
func parseRepo(s string) (owner, name string, err error) {
	parts := strings.FieldsFunc(strings.TrimSuffix(strings.TrimSpace(s), ".git"), func(r rune) bool {
		return r == '/' || r == ':'
	})
	if len(parts) < 2 {
		return "", "", fmt.Errorf("cannot read owner/name from %q", s)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}
