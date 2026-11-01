# the-eye

repo_name: the-eye
repo_summary: Go hub that runs Claude Code sessions against a repo's GitHub epics and sub-issues
verify: go vet ./... && go test ./...
worktree_root: ../the-eye-worktrees
shared_files: go.mod, go.sum

## all
- Every change goes through a PR into main, and the PR closes its sub-issue. Workers never close issues.
- Only internal/herdr calls herdr. Keep the hub on localhost.
- Keep changes small and reuse what internal/ already has (README "Layout").

## orchestrator
- Plan and phase order: the plan document at the repo root and the phase epics on GitHub (label `epic`).

## executor
- Checks before DONE: gofmt -l . prints nothing; go vet ./...; go test ./...

## reviewer
- Run the executor checks in the task's worktree.
