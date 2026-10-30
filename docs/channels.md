# Channel capability (spike #36)

## Result

With the `AfterInitialize` hook set, an mcp-go stdio server with `mcp-go v1.5.0` returns this in its `initialize` response:

```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"experimental":{"claude/channel":{}}},"serverInfo":{"name":"channel-spike","version":"0.0.1"}}}
```

Reproduce with the spike server (`cmd/channelspike`):

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"probe","version":"0"}}}' | go run ./cmd/channelspike
```

## What this shows

- mcp-go can emit `experimental.claude/channel` from the `AfterInitialize` hook, so the relay design (#37) can rely on the server side of it.

## What this does not show

- That Claude Code accepts the capability and delivers channel events into a session. That needs a live session started with `--dangerously-load-development-channels` and a claude.ai login (#38). Not done here.
