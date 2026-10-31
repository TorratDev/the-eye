package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/TorratDev/the-eye/internal/spawner"
)

// openPR opens the PR for a task the reviewer passed (docs/pr-owner.md), from the branch the
// executor pushed. If a PR is already open for that branch it returns that one, so a retry never
// opens a second.
func openPR(gh func(args ...string) (string, error), repo string, ex spawner.ExecutorDispatch, done, verdict string) (string, error) {
	out, err := gh("pr", "list", "-R", repo, "--head", ex.Branch, "--state", "open", "--json", "url", "-q", ".[0].url")
	if err != nil {
		return "", err
	}
	if url := strings.TrimSpace(out); url != "" {
		return url, nil
	}
	title := ex.Objective
	if len(title) > 100 {
		title = title[:100]
	}
	body := fmt.Sprintf("Closes #%d\n\n%s\n\nExecutor: %s\n\nReviewer: %s\n\nOpened by the hub after review.", ex.Issue, ex.Objective, done, verdict)
	out, err = gh("pr", "create", "-R", repo, "--head", ex.Branch, "--title", title, "--body", body)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return lines[len(lines)-1], nil // gh prints the new PR's URL last
}

// ghRun runs gh, which holds the GitHub login.
func ghRun(args ...string) (string, error) {
	out, err := exec.Command("gh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh %s %s: %w: %s", args[0], args[1], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
