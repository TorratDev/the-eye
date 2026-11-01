# the-eye: Claude Code orchestrator hub

A local Go hub that runs Claude Code sessions against a repo's GitHub epics and sub-issues. GitHub is the source of truth: epics are plans, sub-issues are tasks, and labels and "blocked by" relations order the work. The hub keeps local task state, launches role sessions in herdr-core (`hc`) terminals, which the herdr desktop app shows, applies their reports, and shows every session's state in a web page.

Phase 9 turns the watcher into a minimal session app: start an epic, read a session's transcript, and send it messages, all on localhost. Until it lands, the UI only watches.

The plan is in [`Claude Code Orchestrator — Plan.md`](Claude%20Code%20Orchestrator%20%E2%80%94%20Plan.md). Design notes and decisions are in [`docs/`](docs).

## Roles

| Role | Model | Writes code | Pane |
| --- | --- | --- | --- |
| Orchestrator | claude-haiku-5-5 | No | one |
| Investigator | claude-haiku-5-5 | No | three (A, B, C) |
| Executor | claude-haiku-5-5 | Yes, in its own worktree | three |
| Reviewer | claude-sonnet-5-5 | No | two |

Each role's prompt is in [`prompts/`](prompts). The hub fills the placeholders from a per-repo config in `repos/<repo>.md` (see [`docs/repo-config.md`](docs/repo-config.md)); `repos/rust-mm.md` is the first.

## Commands

```bash
go run ./cmd/hub plan <epic>          # sync, check the baseline, serve, start the orchestrator and send it the epic
go run ./cmd/hub sync <epic>          # print the epic's sub-issues and which are ready
go run ./cmd/hub serve                # watcher page, POST /hooks, /events, POST /report on 127.0.0.1:8080
go run ./cmd/hub serve --epic <n>     # also poll the epic's cloud worker reports every 30 s
go run ./cmd/hub mcp --role <role>    # the report tool over stdio, for one session
```

`hub plan` takes `--config` (default `repos/<name>.md`), `--cwd` (the target repo's checkout, where the panes start) and the `serve` flags. Run it from this repo, since it reads `prompts/` and `.hub/` relative to the current directory.

`hub serve` holds the state store (`.hub/state.db`, gitignored) exclusively, so `hub sync` cannot run while it is up. Keep `serve` on localhost.

## Requirements

- Go 1.26 (see `go.mod`)
- `gh`, logged in to the target repo
- herdr-core `hc` 0.1.16 (`herdr.PinnedVersion`), from the herdr desktop app, with its core running (`hc server status`)
- Claude Code with a claude.ai login, and the Ponytail plugin enabled
- RTK (`rtk gain` must print its savings report)
- `hub` on PATH, since `--mcp-config` starts the report tool with `hub mcp`: `go install ./cmd/hub` from this repo

## Layout

- `cmd/hub`: the entry point (`sync`, `serve`, `mcp`).
- `internal/hub`: GitHub sync, the dependency graph, atomic claims, and the SQLite state store.
- `internal/spawner`: one named `hc` terminal per role slot (`e<epic>-<slot>`); the launch line per role; worktrees; batch cap; idle-gated sends; the login and Ponytail/RTK checks.
- `internal/herdr`: the only code that calls herdr-core (`hc` 0.1.16).
- `internal/ingest`: `POST /hooks`, which stores hook events with their role and pane.
- `internal/session`: the final-line parser and the session state model.
- `internal/transcript`: reads a session's transcript (`transcript_path`) into turns.
- `internal/report`: the `report` tool, both the store-side and the remote (stdio) form.
- `internal/live`, `internal/ui`: SSE stream at `/events` and the watcher page (htmx).
- `internal/decision`: parses the orchestrator's `POINTS`, `TASK`, `FOLLOWUP` and `ESCALATE` lines and acts on them through injected effects.
- `internal/cloud`: reads cloud workers' GitHub comments and applies them, with claims in view.
- `internal/config`, `internal/prompts`: per-repo config, and prompt rendering with a check that every placeholder is filled.

## Status

Each phase is tracked as an epic. "Tested" means the Go tests pass; "live" means run against real Claude Code, herdr or GitHub.

| Phase | Epic | Code | Live |
| --- | --- | --- | --- |
| 1. Hub skeleton | #1 | done | `sync` run against this repo |
| 2. Spawner | #8 | done, tested | `hub plan` laid out all four role tabs and nine panes in herdr 0.9.5 and launched the orchestrator (#99); worktrees not yet run |
| 3. Prompts | #17 | done, tested | rust-mm config renders all four prompts |
| 4. Hooks and report tool | #25 | done, tested | the orchestrator's hooks reached `POST /hooks` and its `report` call was commented on the sub-issue (#99) |
| 5. Web UI | #31 | done, tested | page loaded with curl; not loaded in a browser |
| 6. Channel relay and orchestrator | #35 | superseded | the relay ran live (#105, #146), then `hc send` replaced it (#155, `docs/channels.md`) |
| 7. Cloud workers | #40 | done, tested | `serve --epic 95` polled GitHub, picked up a hand-posted BLOCKED report on #104, showed the alert and set the state (#104) |
| 8. One-command epic run | #95 | done, tested | not run live yet (see open items) |
| 9. Minimal session app | #96 | done, tested | not run in a browser yet; the page starts an epic, reads and messages sessions, answers prompts |
| 10. Sessions on herdr-core (`hc`) | #152 | done, tested | throwaway epics #158 and #162 ran end to end on `hc` terminals (#154) with `hc send` forwards (#155); with the herdr app open, an epic's sessions share an `epic <n>` space (#156) |

Open items:
- `hub plan` acts on the orchestrator's decisions from its `Stop` event (`last_assistant_message`, see `docs/hooks.md`). Results that need a human show on the page as a `hub` / `decisions` row. Not yet run live.
- Cloud workers' report contract is in the worker prompts (`docs/cloud-reports.md`). Polling ran live against GitHub (#104); no real cloud worker has posted a report yet.
- The hub opens a PR when a reviewer passes a task; the Progress list links each one (#113). Not yet run live.
- `hub plan` and the page's epic form run one epic per `serve` process; restart `serve` to run another.
- Session resume matches a slot's terminal by name: a session whose terminal is gone (an hc restart) is resumed with `--resume` on the next work for that slot.

## Working rules

- Every change goes through a PR into `main`, and the PR closes its sub-issue.
- Workers never close issues; the merge does.
- Nothing reads or writes a ROM, a dump or extracted game data. See the legal policy in the rust-mm repo (`docs/legal.md`).
