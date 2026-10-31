# Hook ingestion

Claude Code hooks post to the hub's `POST /hooks` (`internal/ingest`). The hub stores each event in the `events` table with the role and pane it came from.

## What a hook sends

- Body: the hook's JSON input. The handler reads `hook_event_name` and `session_id` from it.
- Headers: `X-Hub-Role` and `X-Hub-Pane`. The hook command copies them from `HUB_ROLE` and `HUB_PANE`, which the launcher sets for each pane.

The launcher registers the hook command for every session with `--settings` (`spawner.hookSettings`), so nothing goes in the user's or the repo's settings. The hook command:

```bash
curl -s -m 5 -X POST "${HUB_URL:-http://127.0.0.1:8080}/hooks" -H "X-Hub-Role: $HUB_ROLE" -H "X-Hub-Pane: $HUB_PANE" \
  -H "Content-Type: application/json" --data-binary @- >/dev/null || true
```

It never fails, so a hub that is down does not block the session.

## What is stored

`Stop`, `Notification`, `PostToolUse` and `PostToolUseFailure`. The last one is stored too, because the state model reads it as an error. Any other hook name is refused with 400, and so is an unknown role or a missing pane.

## Not decided here

- The handler does not check that the pane is one the hub launched. A pane the hub doesn't know gets stored like any other.

## The orchestrator's reply

`decision.Act` reads the orchestrator's reply from the `Stop` payload's `last_assistant_message`. It holds the text of the turn's last assistant message, the same text as the last assistant entry in the transcript at `transcript_path` (checked in the #99 live run). Nothing has to parse the transcript to get the decision lines.

The transcript (`transcript_path`, JSONL, one entry per line) is still the source for a session's full conversation: entries with `"type": "assistant"` or `"type": "user"` carry a `message` with `content` blocks (`text`, `tool_use`, `tool_result`). Other entry types are Claude Code's bookkeeping.

`Stop` payload from the live run (Claude Code 2.1.316; the reply is cut here):

```json
{
  "session_id": "cea7c612-66bb-4763-abb0-860d5dfe3e42",
  "transcript_path": "~/.claude/projects/-home-torrat-Documents-github-the-eye/cea7c612-66bb-4763-abb0-860d5dfe3e42.jsonl",
  "cwd": "/home/torrat/Documents/github/the-eye",
  "prompt_id": "6d6f907c-6258-447c-ac6c-ba975069ad36",
  "permission_mode": "auto",
  "effort": {"level": "medium"},
  "hook_event_name": "Stop",
  "stop_hook_active": false,
  "last_assistant_message": "Epic #117 is a throwaway smoke test … Decision for the hub:\n\n```\nPOINTS #118: A= B= C=\n```\n\nNo `TASK`, `FOLLOWUP`, or `ESCALATE` lines. …",
  "background_tasks": [],
  "session_crons": []
}
```

The orchestrator may put its decision lines inside a code fence, as here. `decision.Route` reads line by line and skips the fence. The full reply is the fixture `liveReply` in `internal/decision/decision_test.go`.

The idle `Notification` (`"notification_type": "idle_prompt"`, "Claude is waiting for your input") is dropped at ingest. Other notification types, such as permission prompts, are stored.
