package spawner

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree returns the executor's checkout for sub-issue n, creating branch <n>-<slug> from
// origin/main on the first run. A REVISE run finds the existing worktree and branch and reuses them.
func Worktree(repo string, n int, slug, path string) (string, error) {
	path = filepath.Clean(path)
	branch := fmt.Sprintf("%d-%s", n, slug)
	if _, err := git(repo, "fetch", "--quiet", "origin"); err != nil {
		return "", err
	}
	existing, err := worktrees(repo)
	if err != nil {
		return "", err
	}
	if _, ok := existing[path]; ok {
		return path, nil
	}
	if _, err := git(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		_, err = git(repo, "worktree", "add", path, branch)
		return path, err
	}
	_, err = git(repo, "worktree", "add", "-b", branch, path, "origin/main")
	return path, err
}

// Leftovers lists worktrees other than the main checkout. The hub reports them before a batch
// starts and never deletes them.
func Leftovers(repo string) ([]string, error) {
	all, err := worktrees(repo)
	if err != nil {
		return nil, err
	}
	main, err := git(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	main = filepath.Clean(filepath.FromSlash(main))
	var out []string
	for path := range all {
		if path != main {
			out = append(out, path)
		}
	}
	return out, nil
}

// worktrees maps each worktree path to its branch from `git worktree list --porcelain`.
func worktrees(repo string) (map[string]string, error) {
	out, err := git(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	var path string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), " ")
		switch k {
		case "worktree":
			path = filepath.Clean(filepath.FromSlash(v)) // git prints forward slashes on Windows
			m[path] = ""
		case "branch":
			m[path] = strings.TrimPrefix(v, "refs/heads/")
		}
	}
	return m, sc.Err()
}

// git runs one git command in repo and returns its trimmed stdout.
func git(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
