# rust-mm

repo_name: rust-mm
repo_summary: Rust reimplementation of a commercial Nintendo DS game, with reverse-engineering tools
verify: cargo test --workspace
worktree_root: ../rust-mm-worktrees
shared_files: Cargo.lock

## all
- Never commit the game, its dumps, extracted assets (sprites, text, maps, music, data tables decoded from the ROM), saves, savestates, or screenshots of game graphics. See docs/legal.md.
- Tests that need game data read the user's dump through RESCUE_ROM and skip cleanly without it. Fixtures are generated at test time into target/rom-fixtures, never committed.
- Use the neutral name: crates and binaries use the `rescue` prefix. No trademarks in names.
- Treat pmd-red as read-only reference: read and cite it, do not copy its code or data.
- Mark facts as verified (with file, address, trace or command) or as hypotheses. Record RE findings in docs/re/.
- Decompile with ghidra-cli only, per CLAUDE.md. Do not drive Ghidra by hand.
- Before committing, run git status and read the diff for anything .gitignore might miss.

## orchestrator
- Plans and roadmap order: docs/roadmap.md, docs/plans/README.md.

## investigator
- Read the code and docs your points need: docs/re/, docs/symbols.md, docs/verification.md, docs/references.md.
- Use ghidra-cli against the rescue-arm9 project for addresses and xrefs (docs/ghidra-setup.md).

## executor
- Pinned toolchain: rust-toolchain.toml (1.97.0). Keep rust-version in Cargo.toml in step with it.
- Checks before DONE: cargo fmt --all -- --check; cargo clippy --workspace --all-targets -- -D warnings; sh scripts/check-deps.sh; sh scripts/check-determinism.sh (no floats, HashMap or clocks in sim and data).
- Save files and savestates stay local.

## reviewer
- Legal check first: a forbidden file in the diff is an automatic ESCALATE.
- Run the checks in the executor section in the task's worktree.

## resources
| name | lock |
| --- | --- |
| rescue-arm9 Ghidra project | exclusive: one ghidra-cli session at a time |
| RESCUE_ROM dump | shared read-only |
