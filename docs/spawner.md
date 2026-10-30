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
