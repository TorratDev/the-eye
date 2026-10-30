# Hook ingestion

Claude Code hooks post to the hub's `POST /hooks` (`internal/ingest`). The hub stores each event in the `events` table with the role and pane it came from.

## What a hook sends

- Body: the hook's JSON input. The handler reads `hook_event_name` and `session_id` from it.
- Headers: `X-Hub-Role` and `X-Hub-Pane`. The hook command copies them from `HUB_ROLE` and `HUB_PANE`, which the launcher sets for each pane.

Example hook command (for `Stop`):

```bash
curl -s -X POST "$HUB_URL/hooks" -H "X-Hub-Role: $HUB_ROLE" -H "X-Hub-Pane: $HUB_PANE" \
  -H "Content-Type: application/json" --data-binary @-
```

## What is stored

`Stop`, `Notification`, `PostToolUse` and `PostToolUseFailure`. The last one is stored too, because the state model reads it as an error. Any other hook name is refused with 400, and so is an unknown role or a missing pane.

## Not decided here

- The handler does not check that the pane is one the hub launched. A pane the hub doesn't know gets stored like any other.
- Nothing serves the handler yet. The hub has no `serve` command.
