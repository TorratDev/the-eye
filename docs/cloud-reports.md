# Cloud reports: polling, not webhooks

## Decision

The hub polls GitHub for cloud workers' comments and PRs. It does not take webhooks.

## Why

| | Polling | Webhook |
| --- | --- | --- |
| Reachability | Needs only outbound HTTPS from the local hub | GitHub must reach the hub, so it needs a public endpoint or a tunnel. Hosting the hub is out of scope. |
| Latency | Up to one poll interval (for example 30 seconds) | Near instant |
| Rate limits | Per-issue reads count against the GitHub API limit. A few sub-issues per epic is far below it. | None for receiving |
| Setup | Works with the existing `gh` login | Webhook secret, public URL, and a delivery check |

Polling fits this project: a few issues, a local hub, and no hosting. A cloud worker reports minutes apart, so the extra latency is acceptable.

## When to switch

Switch to webhooks only if polling is too slow for the orchestrator's decisions, or the API rate limit becomes a problem. Hosting the hub would be needed first.
