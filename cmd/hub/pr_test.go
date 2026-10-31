package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/TorratDev/the-eye/internal/spawner"
)

// A PASS pushes the task's branch from its worktree and opens one PR from it with Closes #n; when
// one is already open for the branch, it is returned and no second one is created.
func TestOpenPR(t *testing.T) {
	ex := spawner.ExecutorDispatch{Issue: 21, Branch: "21-flag", Worktree: "/wt/21", Objective: "Add the flag"}
	open := ""
	var calls [][]string
	gh := func(args ...string) (string, error) {
		calls = append(calls, args)
		if args[1] == "list" {
			return open + "\n", nil
		}
		open = "https://github.com/o/r/pull/7"
		return "Creating pull request for 21-flag into main\n\n" + open + "\n", nil
	}
	var pushes [][]string
	git := func(args ...string) (string, error) { pushes = append(pushes, args); return "", nil }
	url, err := openPR(git, gh, "o/r", ex, "DONE: added; verification: ok; branch: 21-flag", "PASS: fine")
	if err != nil || url != "https://github.com/o/r/pull/7" {
		t.Fatalf("openPR = %q, %v", url, err)
	}
	if len(pushes) != 1 || strings.Join(pushes[0], " ") != "-C /wt/21 push -u origin 21-flag" {
		t.Fatalf("push %q", pushes)
	}
	create := calls[1]
	body := create[slices.Index(create, "--body")+1]
	if create[1] != "create" || create[slices.Index(create, "--head")+1] != "21-flag" || !strings.HasPrefix(body, "Closes #21\n") || !strings.Contains(body, "PASS: fine") {
		t.Fatalf("gh %q", create)
	}
	calls = nil
	if url, err := openPR(git, gh, "o/r", ex, "DONE: x", "PASS: y"); err != nil || url != open || len(calls) != 1 {
		t.Fatalf("retry: %q, %v, calls %q", url, err, calls)
	}
}
