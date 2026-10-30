# Spawner: herdr calls

herdr is pinned to **0.9.5** (`herdr.PinnedVersion`). The spawner reaches it only through `internal/herdr`, and `TestPinnedVersion` fails when the installed version drifts.

| Wrapper | herdr command | Used for |
| --- | --- | --- |
| `Version` | `herdr --version` | Pin check |
| `WorkspaceList` | `herdr workspace list` | Find an epic's workspace |
| `WorkspaceCreate` | `herdr workspace create --cwd --label --no-focus` | One workspace per epic |
| `TabCreate` | `herdr tab create --workspace --cwd --label --no-focus` | One tab per role |
| `PaneSplit` | `herdr pane split <pane> --direction --cwd` | Extra panes for role sessions |
| `PaneRun` | `herdr pane run <pane> <command>` | Start the shell command in a pane |
| `AgentRead` | `herdr agent read <target> --source recent --lines N` | Secondary state signal; plain text |

Notes:
- The JSON-producing commands print an envelope: `{"id":…,"result":{…}}` on success, `{"id":…,"error":{"code","message"}}` on failure. Seen for `workspace list`, `workspace get`, `agent list` and `pane list`, and the error case for `agent read`; the field names for create, split and run follow the bundled API schema (`herdr api schema --json`).
- Only read-only calls were run against the live session. Create, split and run were not, so they are untested.

## Hook order (observed)

Checked with one live `claude -p` session on claude-haiku-5-5, with the hub's probe hooks added through `--settings` and the user's own hooks (RTK `PreToolUse` on Bash, the herdr `SessionStart` hook, the Ponytail plugin) left as they are.

Probe hooks: `scripts/hookprobe.ps1` appends the hook name, the Bash command it received and the response length to the file named by `HOOKPROBE_LOG`. It is registered for `PreToolUse`, `PostToolUse` (both matcher `Bash`) and `Stop`.

Command used (run from a scratch directory, with `<probe>` the directory holding `settings.json` and `probe.ps1`):

```bash
claude -p "Run the bash command: git -C <repo> status. Then reply with only the word done." \
  --settings <probe>/settings.json --model claude-haiku-5-5 --allowedTools Bash --output-format text </dev/null
```

Observed (second run, `git status`):

| Time | Hook | Bash command it received |
| --- | --- | --- |
| 18:06:14 | hub `PreToolUse` | `git -C … status` (original) |
| 18:06:16 | hub `PostToolUse` | `rtk git -C … status` (rewritten) |
| 18:06:17 | hub `Stop` | — |

What this shows:
- RTK's rewrite is in effect by the time `PostToolUse` runs, so anything the hub reads from `PostToolUse` sees the rewritten command, not the one the prompt named.
- The hub's `PreToolUse` saw the original command. Both `PreToolUse` hooks get the same input, so the run order of RTK's and the hub's `PreToolUse` is not recorded here, and the timestamps alone do not settle it. Do not rely on either running first.
- The first run (`echo`) showed the same pattern, except the command was not rewritten. RTK rewrites only some commands.

Not checked: the Ponytail hooks and the herdr `SessionStart` hook were not logged, and the order of the hub's `Notification` hook is untested because no hub hook exists yet.

## Ponytail context overhead (one sample each)

Same sub-issue (#18: check the four role prompts against the eight-part shape and add a test), run once with Ponytail enabled and once with it disabled through `--settings`, on claude-haiku-5-5, with `claude -p --output-format json` in a fresh worktree from `origin/main`.

| Run | Input tokens | Cache write | Cache read | Output tokens | Turns | Cost (USD) | Time (s) | Test file lines |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Ponytail on (default) | 14 | 29,619 | 245,858 | 4,090 | 7 | 0.0104 | 33.3 | 89 |
| Ponytail off (`enabledPlugins` false) | 18 | 27,899 | 322,743 | 5,590 | 9 | 0.0116 | 38.2 | 102 |

What the numbers show, and what they don't:
- With Ponytail on, the run was cheaper and used fewer turns, and the test file was smaller.
- The off run's test caught a real drift: `prompts/orchestrator.md` names section 5 "Judgment calls", not "Procedure". The on run accepted that title by widening its check, so the drift went unreported. Ponytail's "smallest change" rule may have encouraged that, but one sample cannot show it.
- One sample per condition is not enough to set a default. The default level per role is not decided yet. Ponytail stays on everywhere until more runs say otherwise.
- The off setting was applied through `--settings`. The run did not check that the plugin was absent from the session context.
