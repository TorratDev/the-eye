package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/TorratDev/the-eye/internal/spawner"
)

// A PASS opens one PR from the task's branch with Closes #n; when one is already open for the
// branch, it is returned and no second one is created.
func TestOpenPR(t *testing.T) {
	ex := spawner.ExecutorDispatch{Issue: 21, Branch: "21-flag", Objective: "Add the flag"}
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
	url, err := openPR(gh, "o/r", ex, "DONE: added; verification: ok; branch: 21-flag", "PASS: fine")
	if err != nil || url != "https://github.com/o/r/pull/7" {
		t.Fatalf("openPR = %q, %v", url, err)
	}
	create := calls[1]
	body := create[slices.Index(create, "--body")+1]
	if create[1] != "create" || create[slices.Index(create, "--head")+1] != "21-flag" || !strings.HasPrefix(body, "Closes #21\n") || !strings.Contains(body, "PASS: fine") {
		t.Fatalf("gh %q", create)
	}
	calls = nil
	if url, err := openPR(gh, "o/r", ex, "DONE: x", "PASS: y"); err != nil || url != open || len(calls) != 1 {
		t.Fatalf("retry: %q, %v, calls %q", url, err, calls)
	}
}
