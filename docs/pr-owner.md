# Who opens the PR after PASS

## Decision

The hub opens the PR. The executor commits in its worktree and reports `DONE`; it neither pushes nor runs `gh pr create`. After the reviewer's PASS, the hub pushes the branch from the task's worktree and opens the PR (changed after the #139 live run: the executor waited for a go-ahead to push that nothing sent).

## Comparison

| | Executor opens the PR | Hub opens the PR |
| --- | --- | --- |
| GitHub write rights | Each executor needs PR rights, in addition to push | Only the hub needs push and PR rights |
| Failure handling | A failed `gh pr create` happens inside a session that may be idle or gone | The hub retries, records the failure in its store, and reports it |
| Audit trail | The PR URL is only in the executor's reply | The hub stores the PR URL against the sub-issue and comments on the issue |
| PR body | Written by each executor, so the format can drift | Written by the hub from the task and the DONE line, with `Closes #<n>` |

The hub wins on failure handling and audit, and it keeps the PR body in one format. Since the #139 live run the hub also pushes, so executors need neither push nor PR rights.

## What changed

- `prompts/executor.md`: step 8 says not to push or open a PR, since the hub does both after PASS; the DONE line carries `branch:` instead of `PR:`.
- The plan's open question is marked decided.
