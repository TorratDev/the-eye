# Who opens the PR after PASS

## Decision

The hub opens the PR. The executor pushes its branch and reports `DONE`. It does not run `gh pr create`.

## Comparison

| | Executor opens the PR | Hub opens the PR |
| --- | --- | --- |
| GitHub write rights | Each executor needs PR rights, in addition to push | Only the hub needs PR rights; executors still push the branch |
| Failure handling | A failed `gh pr create` happens inside a session that may be idle or gone | The hub retries, records the failure in its store, and reports it |
| Audit trail | The PR URL is only in the executor's reply | The hub stores the PR URL against the sub-issue and comments on the issue |
| PR body | Written by each executor, so the format can drift | Written by the hub from the task and the DONE line, with `Closes #<n>` |

The hub wins on failure handling and audit, and it keeps the PR body in one format. Executors still need push rights, so the saving on permissions is only the PR-create right.

## What changed

- `prompts/executor.md`: step 9 says to push and not run `gh pr create`; the DONE line carries `branch:` instead of `PR:`; the worktree is removed after the hub reports the PR open.
- The plan's open question is marked decided.
