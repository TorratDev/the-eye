# the-eye: Claude Code orchestrator hub

A local Go hub that works through a repo's GitHub epics with a team of Claude Code sessions. GitHub is the source of truth: an epic is the plan, its sub-issues are the tasks, and "blocked by" relations order them. The hub plans with an orchestrator session, hands tasks to executors in their own worktrees, has reviewers check the work, and opens a PR for each task that passes.

Sessions run as herdr-core (`hc`) terminals, so they keep running when nothing is watching. You follow them in the hub's web page, and in the herdr desktop app when it is open.

The plan is in [`Claude Code Orchestrator — Plan.md`](Claude%20Code%20Orchestrator%20%E2%80%94%20Plan.md). Design notes and decisions are in [`docs/`](docs).

## How a run works

`hub plan <epic>` carries one epic from planning to PRs:

1. **Sync**: reads the epic's sub-issues from GitHub and which are ready (open, unclaimed, every blocker closed, not on a dependency cycle).
2. **Checks**: refuses to start without RTK, the Ponytail plugin, a valid repo config, or a claude.ai login.
3. **Serve**: starts the web page and the endpoints sessions report to, on `127.0.0.1:8080`.
4. **Orchestrator**: starts it in an `hc` terminal and sends it the epic and its ready sub-issues. It answers with decisions: `POINTS` for investigators, `TASK` for executors, `FOLLOWUP` or `ESCALATE`.
5. **Executors**: for each `TASK`, the hub claims the sub-issue on GitHub (`hub:claimed`), makes a worktree, starts an executor and sends it the task. The executor commits, runs the repo's checks and ends with a `DONE` or `BLOCKED` line.
6. **Review**: a `DONE` goes to a reviewer, which answers `PASS`, `REVISE` (back to the executor, and to the orchestrator after three), `SPLIT` or `ESCALATE`.
7. **PR**: on `PASS`, the hub pushes the branch and opens a PR that closes the sub-issue. Merging stays with you.

Throughout, the hub forwards each worker's result line and newly ready sub-issues to the orchestrator with `hc send`, which reaches it even mid-turn. Sessions start only when their slot gets work, and a session that lost its terminal is resumed on its next task.

## Quick start

```bash
go install ./cmd/hub          # sessions start the report tool as `hub mcp`, so it must be on PATH
hc server status              # the hc core must be running (the herdr app starts it)
hub sync <epic>               # see which sub-issues are ready
hub plan <epic>               # run the epic; the page opens in your browser
```

Run `hub` from this repo: it reads `prompts/` and keeps its state in `.hub/` relative to the current directory. Point `--cwd` at the target repo's checkout when it lives elsewhere.

## Commands

| Command | What it does |
| --- | --- |
| `hub plan [--repo owner/name] [--config repos/<name>.md] [--cwd dir] <epic>` | Runs the epic as above. Also takes the `serve` flags. |
| `hub sync [--repo owner/name] <epic>` | Prints the epic's sub-issues, their blockers and which are ready. Starts nothing. |
| `hub serve [--repo owner/name] [--epic n] [--addr host:port] [--poll 30s]` | The page and endpoints only; with `--epic`, also polls the epic's cloud worker reports. The page's form can start an epic. |
| `hub mcp [--role role] [--url hub-url]` | The `report` tool over stdio for one session (sessions start this themselves). |

`--repo` defaults to the `origin` remote, `--config` to `repos/<repo name>.md`. One `serve` process runs one epic; restart it to run another. `serve` holds `.hub/state.db` (gitignored) exclusively, so `hub sync` can't run while it's up. Keep `serve` on localhost: the page can start sessions and type into them.

## Watching a run

- **Web page** (`http://127.0.0.1:8080`): progress per sub-issue with PR links, every session's state, and errors and permission prompts with Allow and Deny. Each session has a page with its transcript and a box to message it. Updates live over SSE.
- **herdr app**: when its window is open, an epic's sessions share a space named `epic <n>`, one tab each, with their status in the Agents list.
- **`hc`**: `hc ls` lists the sessions, named `e<epic>-<slot>` (for example `e158-executor-1`); `hc read e158-executor-1` shows one's screen.

## Roles

| Role | Model | Writes code | Sessions |
| --- | --- | --- | --- |
| Orchestrator | claude-haiku-5-5 | No | one |
| Investigator | claude-haiku-5-5 | No | three (A, B, C) |
| Executor | claude-haiku-5-5 | Yes, in its own worktree | three |
| Reviewer | claude-sonnet-5-5 | No | two |

Each role's prompt is in [`prompts/`](prompts). The hub fills its placeholders from the target repo's config in `repos/<repo>.md` (see [`docs/repo-config.md`](docs/repo-config.md)); there are configs for `rust-mm` and this repo.

## Requirements

- Go 1.26 (see `go.mod`)
- `gh`, logged in with access to the target repo
- herdr-core `hc` 0.1.16 (`herdr.PinnedVersion`) with its core running; it comes with the herdr desktop app, which is optional to have open
- Claude Code with a claude.ai login, and the Ponytail plugin enabled
- RTK (`rtk gain` must print its savings report)
- `hub` on PATH (`go install ./cmd/hub`)

## Layout

- `cmd/hub`: the CLI (`plan`, `sync`, `serve`, `mcp`) and the epic run: forwarding, reviews, decisions and PRs.
- `internal/hub`: GitHub sync, the dependency graph, atomic claims, and the SQLite state store.
- `internal/spawner`: one named `hc` terminal per role slot, the launch line per role, worktrees, the batch cap, idle-gated dispatches, the epic's app space, and the login, Ponytail and RTK checks.
- `internal/herdr`: the only code that calls herdr-core (`hc`) and the herdr app.
- `internal/ingest`: `POST /hooks`, which stores hook events with their role and pane.
- `internal/session`: the final-line parser and the session state model.
- `internal/transcript`: reads a session's transcript (`transcript_path`) into turns.
- `internal/report`: the `report` tool, both the store-side and the remote (stdio) form.
- `internal/live`, `internal/ui`: the SSE stream at `/events` and the web pages (htmx).
- `internal/decision`: parses the orchestrator's `POINTS`, `TASK`, `FOLLOWUP` and `ESCALATE` lines and acts on them.
- `internal/cloud`: reads cloud workers' GitHub comments and applies them, with claims in view.
- `internal/config`, `internal/prompts`: per-repo config, and prompt rendering with a check that every placeholder is filled.

## Working rules

- Every change goes through a PR into `main`, and the PR closes its sub-issue.
- Workers never close issues; the merge does.
- Nothing reads or writes a ROM, a dump or extracted game data. See the legal policy in the rust-mm repo (`docs/legal.md`).
