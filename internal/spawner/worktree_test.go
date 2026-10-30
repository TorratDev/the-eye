package spawner

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// A REVISE run must reuse the same worktree and branch; a leftover worktree must be reported.
func TestWorktreeReviseAndLeftovers(t *testing.T) {
	remote := t.TempDir()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "--bare", remote},
		{"init", "-q", "-b", "main", repo},
		{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"-C", repo, "remote", "add", "origin", remote},
		{"-C", repo, "push", "-q", "origin", "main"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	path := filepath.Join(t.TempDir(), "wt")

	if _, err := Worktree(repo, 12, "slug", path); err != nil {
		t.Fatal(err)
	}
	left, err := Leftovers(repo)
	if err != nil || len(left) != 1 {
		t.Fatalf("Leftovers = %v, %v; want the one executor worktree", left, err)
	}

	// REVISE: same path comes back, no second branch or worktree is made
	if _, err := Worktree(repo, 12, "slug", path); err != nil {
		t.Fatal(err)
	}
	all, _ := worktrees(repo)
	if all[path] != "12-slug" || len(all) != 2 {
		t.Errorf("after REVISE worktrees = %v, want %s on 12-slug", all, path)
	}
}
