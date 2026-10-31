package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/TorratDev/the-eye/internal/spawner"
)

// openPR pushes a task the reviewer passed from its worktree and opens its PR (docs/pr-owner.md).
// If a PR is already open for the branch it returns that one, so a retry never opens a second.
func openPR(git, gh func(args ...string) (string, error), repo string, ex spawner.ExecutorDispatch, done, verdict string) (string, error) {
	if _, err := git("-C", ex.Worktree, "push", "-u", "origin", ex.Branch); err != nil {
		return "", err
	}
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
func ghRun(args ...string) (string, error) { return runTool("gh", args...) }

// gitRun runs git.
func gitRun(args ...string) (string, error) { return runTool("git", args...) }

func runTool(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
