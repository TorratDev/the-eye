# Channels (#36, #105, #155)

The hub no longer uses Claude Code channels. It sends the orchestrator worker results and ready
sub-issues with `hc send` (herdr-core), which reaches a session mid-turn and refuses while the
session waits on a permission prompt, so a forward never answers a dialog.

## Why the channel relay went

The relay worked live (PR #146, epic #147): `hub mcp` declared `claude/channel` and relayed the
hub's `/channel` stream as `notifications/claude/channel`. It cost:

- `--dangerously-load-development-channels server:hub` on the orchestrator's launch, and a
  "Loading development channels" warning on every launch that the hub had to answer.
- Pinning `hub mcp` to the legacy MCP protocol, since claude skips channels on 2026-07-28
  ("no unsolicited notification path").
- A lost message when the relay dropped between the hub's check and its publish.

The #151 spike showed `hc send` does the same job without any of it: a send during a 20-second
tool call reached Claude in the same turn.

## Notes kept from the live runs (claude 2.1.316, mcp-go v1.5.0)

- The channel notification is `notifications/claude/channel` with params `{content, meta?}`; meta
  keys must match `^[a-zA-Z_][a-zA-Z0-9_]*$`.
- A server must declare `experimental["claude/channel"]`; `server.WithExperimental` does it for both
  protocol eras.
