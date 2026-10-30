# Claude Code Orchestrator — Plan

Oct 29, 2026 · @Torrat

## Goal and scope

A local Go hub runs Claude Code sessions against a repo's GitHub epics and sub-issues, and a web UI shows every local and cloud session's state so errors surface fast.

- **Source of truth**: GitHub. Epics are plans, sub-issues are tasks, and labels or Project fields hold status. "Blocked by" relationships order dispatch.
- **Repo-agnostic**: role prompts are generic. Repo specifics come from a per-repo config injected by the hub.
- **Out of scope for the first iteration**: running the hub as a hosted service; a full UI for controlling sessions (the first UI only watches).

## Architecture

GitHub holds the tasks, the Go hub holds state and drives herdr, and sessions report back through hooks and the `report` tool.

&#91;embedded content: architecture · hub, GitHub, herdr roles\]

## Hub vs orchestrator LLM

Everything mechanical moves out of the orchestrator prompt into deterministic Go code; the orchestrator LLM keeps only judgment calls.

| Responsibility | Go hub | Orchestrator LLM |
| --- | --- | --- |
| Platform detection, launcher path | Yes |  |
| herdr workspace, tab and pane setup | Yes |  |
| Start sessions, check model header, idle checks before sending | Yes |  |
| Batch selection: dependencies, file overlap, max 3 | Yes |  |
| Task state (replaces progress.md) | Yes |  |
| Format and send dispatch messages | Yes |  |
| Review queue, REVISE attempt counter | Yes |  |
| Create follow-up issue after 3 REVISE | Yes (mechanics) | Writes the body |
| Worktree and leftover-process cleanup checks | Yes |  |
| Split points across investigators A, B, C |  | Yes |
| Turn findings into a task: Objective, files, verification |  | Yes |
| Decide on SPLIT, ESCALATE, BLOCKED |  | Yes |
| Split epics into sub-issues |  | Yes |

## Roles

Each role gets one herdr tab in the epic workspace; extra sessions of a role are panes in that tab, never a second tab. First iteration runs 3 investigators, 3 executors and 2 reviewers.

| Role | Tab | Panes | Pane names | Model | Writes code |
| --- | --- | --- | --- | --- | --- |
| Orchestrator | orchestrator | 1 | orchestrator | claude-haiku-5-5 | No |
| Investigator | investigators | 3 | investigator-a, -b, -c | claude-haiku-5-5 | No |
| Executor | executors | 3 | executor-1, -2, -3 | claude-haiku-5-5 | Yes, in its own worktree |
| Reviewer | reviewer | 2 | reviewer-1, -2 | claude-sonnet-5-5 | No |

- Model, system prompt and allowed tools are passed per role at launch (`--model`, `--append-system-prompt`, `--allowedTools`).
- Executors are interchangeable; investigators are shared and take a share of one sub-issue's points at a time.

## Session baseline

Every session, in every role, starts with the Ponytail plugin and the RTK hook active; the launcher checks both before the hub dispatches anything.

| Tool | What it does | Setup (user scope, once per machine) | Launcher check |
| --- | --- | --- | --- |
| [Ponytail](https://github.com/DietrichGebert/ponytail) | YAGNI ruleset: reuse existing code, stdlib and platform features before writing new code | `/plugin marketplace add DietrichGebert/ponytail` then `/plugin install ponytail@ponytail`; needs `node` on PATH | Plugin listed, `/ponytail` level set per role |
| [RTK](https://github.com/rtk-ai/rtk) | PreToolUse hook that rewrites Bash commands to compressed output (`git status` → `rtk git status`) | Install binary, `rtk init --global`, register the hook in `~/.claude/settings.json` | `rtk gain` returns stats (not Rust Type Kit) |

- **Compressed output by default**: always work from RTK's compressed output; raw output via `rtk proxy <command>` is a last resort, only when the compressed output cannot verify a specific claim, and the report must say why it was needed.
- **RTK only covers Bash**: Read, Grep and Glob bypass the hook.
- **Ponytail is not a reviewer**: `/ponytail-review` only flags over-engineering, so it complements the reviewer checklist, never replaces it.
- **Hooks coexist**: Ponytail's and RTK's hooks run alongside the hub's Stop, Notification and PostToolUse hooks; verify ordering in the first spawner test.

## Prompt structure

Every worker prompt keeps the same eight-part shape; repo specifics live in a `{{repo_rules}}` block the hub injects from a per-repo config (module dir, docs to read, verify command, layout and legal rules).

1. Identity: role, what it does and doesn't do, what runs in parallel.
2. Startup: stay idle until dispatched.
3. Required reading.
4. Hard rules: scope, legal, layout, safety, style, plus `{{repo_rules}}`.
5. Numbered per-task procedure.
6. Evidence standard: `[verified]` vs `[hypothesis]`, with file and line or test output.
7. Output contract: call the `report` tool, then print exactly one final line as fallback for the Stop hook.
8. Stop and cleanup.

| Role | Final line | Key changes |
| --- | --- | --- |
| Investigator | Findings table: ID, finding, tag, evidence, confidence, suggested task, fits one PR | Report via tool; repo evidence sources from `{{repo_rules}}` |
| Executor | `DONE: …` or `BLOCKED: …` | Create the worktree before editing (fixes step 2/3 order); open PR with `Closes #n` only after PASS; never close issues |
| Reviewer | `PASS`, `REVISE`, `SPLIT` or `ESCALATE` | 2 panes instead of 1, Sonnet instead of Opus; repo checks from `{{repo_rules}}` |
| Orchestrator | Task specs and decisions | Drops all herdr, platform, batching and progress.md sections; keeps planning and judgment |

## Task flow

Workers never close issues: a reviewer PASS leads to a PR with `Closes #n`, and the merge closes the issue.

&#91;embedded content: task flow · one sub-issue from claim to merge\]

The hub runs up to 3 sub-issues per batch, with no shared files and no unmerged dependencies; investigation runs for one sub-issue at a time.

## Reporting and state

Hooks are the primary state signal; herdr is used to spawn sessions and as a secondary signal.

| Source | Carries | Path to hub |
| --- | --- | --- |
| Claude Code hooks: Stop, Notification, PostToolUse | Turn finished, permission prompt, tool errors | POST `/hooks` with role and pane ID from env vars |
| `report` MCP tool | Status and summary of the task | Hub stores it and comments it on the issue |
| Final line in the pane | DONE/BLOCKED or reviewer verdict | Parsed by the Stop hook as fallback |
| herdr agent state | Working, idle, blocked | `herdr agent read` / socket API |
| GitHub | Cloud workers' comments and PRs | Webhook or polling |
| Channel relay | Hub events into the orchestrator session | Go stdio MCP (`mark3labs/mcp-go`) subscribing to `/events` over SSE |

The web UI subscribes to the same SSE stream (Go plus htmx) and shows sessions, states, errors and issue progress.

## Phases

Each phase ends with something runnable; cloud workers come last.

1. **Hub skeleton**: GitHub sync (epic and sub-issues via GraphQL), atomic claiming, state store.
2. **Spawner**: herdr tabs and panes per role, one worktree per executor pane, launch with model, prompt and tools per role.
3. **Prompts**: the four generic role prompts plus the per-repo `{{repo_rules}}` config.
4. **Hooks and report tool**: `/hooks` ingestion, `report` MCP tool, session state model.
5. **Web UI**: SSE plus htmx view of sessions, states, errors and issue progress.
6. **Channel relay and orchestrator session**: push hub events into the orchestrator.
7. **Cloud workers**: report through GitHub, ingested by webhook or polling.

## Risks and open decisions

- **Channels are a research preview**: they need `--dangerously-load-development-channels` and a claude.ai login, and don't register with `-p` or the Agent SDK.
- **mark3labs/mcp-go capability**: confirm `experimental.claude/channel` appears in the `initialize` response (set via the AfterInitialize hook).
- **Usage limits**: 9 parallel sessions (1 + 3 + 3 + 2) will hit subscription limits fast. Start sessions only when a batch needs them.
- **Context overhead**: Ponytail injects its ruleset into every session and subagent; measure token use with and without it on the first epic.
- **Haiku quality**: track the reviewer rejection rate; promote executors to Sonnet if it stays high.
- **Cloud sessions**: no localhost access and limited state visibility; they report through GitHub only.
- **herdr API stability**: pin the version the spawner is tested against.
- **Reference**: Roost (github.com/avesalight/roost) for a working milestone-driven example.

Open questions:

- [ ] Per-repo config format and location (for example `.orchestrator/repo.md`)?
- [x] Should the hub or the executor open the PR after PASS? Decided: the hub (see `docs/pr-owner.md`).
- [ ] Is Sonnet strict enough as reviewer, or do some epics need Opus?
