# Channels (#36, #105)

When `claude` is 2.1.80 or later, the hub launches the orchestrator with
`--dangerously-load-development-channels server:hub` and `HUB_CHANNEL=1`. Its `hub mcp` then relays
the hub's `/channel` stream into the session, so worker results and ready sub-issues reach the
orchestrator mid-turn. With no relay connected the hub types them into the pane once it is idle, as
before.

## What the live run (claude 2.1.316, mcp-go v1.5.0) showed

- claude shows a "WARNING: Loading development channels" dialog on every launch with the flag. The
  hub presses Enter on it (`spawner.acceptChannels`); until then the pane is blocked.
- The server must declare `experimental["claude/channel"]`; `server.WithExperimental` does it for
  both protocol eras (an `AfterInitialize` hook never runs on 2026-07-28, which has no initialize).
- claude skips channels on protocol 2026-07-28 ("no unsolicited notification path"), so `hub mcp`
  offers only the legacy versions for the orchestrator.
- The notification is `notifications/claude/channel` with params `{content, meta?}`; meta keys must
  match `^[a-zA-Z_][a-zA-Z0-9_]*$`.
- claude prints "server:hub · no MCP server configured with that name" at startup for a server from
  `--mcp-config`, then registers the channel anyway once the server connects
  (`~/.cache/claude-cli-nodejs/<project>/mcp-logs-hub/`: "Channel notifications registered").
