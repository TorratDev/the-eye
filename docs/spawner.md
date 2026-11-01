# Spawner: hc calls

The hub runs its sessions as herdr-core (`hc`) terminals, pinned to **0.1.16** (`herdr.PinnedVersion`). It reaches `hc` and the herdr app only through `internal/herdr`, and `TestLive` fails when the installed version drifts. Phase 10 (#152) moved it off the herdr 0.9.5 TUI; the #151 spike has the call-by-call comparison.

| Wrapper | Command | Used for |
| --- | --- | --- |
| `Version` | `hc --version` | Pin check |
| `Run` | `hc run --json --bg --no-parent --rm --name <n> --label k=v -- sh -c <line>` | Start a session's launch line in its own terminal |
| `Get` | `hc get --json <t>` | Is the terminal live, and the session's own status |
| `Read` | `hc read --json --lines N <t>` | The screen, before the session reports too |
| `Keys` | `hc keys --json <t> enter\|esc` | Allow or deny a permission prompt |
| `Send` | `hc send --json <t> <text>` | Forward results and page messages, mid-turn |
| `Wait` | `hc wait --json --timeout S <t>` | Wait for a turn to end |
| `Label` | `hc label --json <t> k=v` | Put a live terminal in the app space |
| `AppSpace` | `herdr-app ls`, `herdr-app do space.new` | Find or make the epic's `epic <n>` space when the window is open |

Notes:
- **Names.** Each role slot's terminal is named `e<epic>-<slot>` (`spawner.Panes`), e.g. `e158-executor-1`. The name is the pane ID everywhere in the hub: store, hooks (`HUB_PANE`), page links. `hc` refuses a second live terminal with the same name (`name_taken`), so a slot can't run twice.
- **Lifetime.** `--rm` removes a terminal when its session exits, so a missing terminal means the session isn't running; the hub resumes it with `--resume <last session of that terminal>` on the slot's next work. `--no-parent` keeps sessions alive, and free of `hc` notes, when the hub itself runs in an `hc` terminal. Sessions survive `hub` restarts and `hc server restart`.
- **Status** is the session's own report (`idle`, `working`, `done`, `blocked` with `kind` and `msg`, `error`); Claude Code reports it without setup. A terminal that hasn't reported yet has no status and isn't ready for input.
- **Sending.** `hc send` pastes and presses Enter once nobody has typed for about three seconds. It reaches a session mid-turn and refuses while the session is blocked on a prompt (`kind` `blocked`), so a forward never answers a dialog. Dispatches to sessions that just started still wait for `idle` (`SendWhenIdle`).
- **Errors.** `hc` prints `{"error":{"kind","message","hint"}}` on stderr with a nonzero exit; the wrapper returns it as `*herdr.Error` (`herdr.NotFound` checks `not_found`).
- **App space.** A terminal labeled `herdr.space=<id>` gets a tab in that space. `space.new` always opens a shell tab first; the hub names it "shell". With no window open, `herdr-app ls` says "no Herdr window" and the hub skips the space.
- **Tested.** `TestLive` runs every `hc` call on a `cat` terminal against the running core. The Claude-specific behavior (status reports, blocked with `kind: permission`, refused sends, mid-turn delivery) was checked in the #151 spike and the live epics #158 and #162.

## Hook order (observed)

Checked with one live `claude -p` session on claude-haiku-5-5, with the hub's probe hooks added through `--settings` and the user's own hooks (RTK `PreToolUse` on Bash, the herdr `SessionStart` hook, the Ponytail plugin) left as they are.

Probe hooks: `scripts/hookprobe.ps1` appends the hook name, the Bash command it received and the response length to the file named by `HOOKPROBE_LOG`. It is registered for `PreToolUse`, `PostToolUse` (both matcher `Bash`) and `Stop`.

Command used (run from a scratch directory, with `<probe>` the directory holding `settings.json` and `probe.ps1`):

```bash
claude -p "Run the bash command: git -C <repo> status. Then reply with only the word done." \
  --settings <probe>/settings.json --model claude-haiku-5-5 --allowedTools Bash --output-format text </dev/null
```

Observed (second run, `git status`):

| Time | Hook | Bash command it received |
| --- | --- | --- |
| 18:06:14 | hub `PreToolUse` | `git -C … status` (original) |
| 18:06:16 | hub `PostToolUse` | `rtk git -C … status` (rewritten) |
| 18:06:17 | hub `Stop` | — |

What this shows:
- RTK's rewrite is in effect by the time `PostToolUse` runs, so anything the hub reads from `PostToolUse` sees the rewritten command, not the one the prompt named.
- The hub's `PreToolUse` saw the original command. Both `PreToolUse` hooks get the same input, so the run order of RTK's and the hub's `PreToolUse` is not recorded here, and the timestamps alone do not settle it. Do not rely on either running first.
- The first run (`echo`) showed the same pattern, except the command was not rewritten. RTK rewrites only some commands.

Not checked: the Ponytail hooks and the herdr `SessionStart` hook (from the herdr TUI's Claude integration; launch lines set `HERDR_ENV=0` against it) were not logged, and the order of the hub's `Notification` hook is untested because no hub hook exists yet.

## Ponytail context overhead (one sample each)

Same sub-issue (#18: check the four role prompts against the eight-part shape and add a test), run once with Ponytail enabled and once with it disabled through `--settings`, on claude-haiku-5-5, with `claude -p --output-format json` in a fresh worktree from `origin/main`.

| Run | Input tokens | Cache write | Cache read | Output tokens | Turns | Cost (USD) | Time (s) | Test file lines |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Ponytail on (default) | 14 | 29,619 | 245,858 | 4,090 | 7 | 0.0104 | 33.3 | 89 |
| Ponytail off (`enabledPlugins` false) | 18 | 27,899 | 322,743 | 5,590 | 9 | 0.0116 | 38.2 | 102 |

What the numbers show, and what they don't:
- With Ponytail on, the run was cheaper and used fewer turns, and the test file was smaller.
- The off run's test caught a real drift: `prompts/orchestrator.md` names section 5 "Judgment calls", not "Procedure". The on run accepted that title by widening its check, so the drift went unreported. Ponytail's "smallest change" rule may have encouraged that, but one sample cannot show it.
- One sample per condition is not enough to set a default. The default level per role is not decided yet. Ponytail stays on everywhere until more runs say otherwise.
- The off setting was applied through `--settings`. The run did not check that the plugin was absent from the session context.
