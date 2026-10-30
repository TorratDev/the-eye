# Per-repo config

The hub fills the `{{...}}` placeholders in the role prompts from one file per repo. The prompts stay the same for every repo.

## Decision

- **Format**: JSON, read with `encoding/json`. The hub is stdlib-only, and Go has no TOML or YAML parser in the standard library.
- **Location**: `.orchestrator/repo.json` at the repo root, tracked in git. It is config, not state, so it is not under `.hub/`, which is gitignored.

## Fields

| Field | Fills | Example |
| --- | --- | --- |
| `repo_name` | `{{repo_name}}` | `rust-mm` |
| `repo_summary` | `{{repo_summary}}` | `Rust reimplementation of a DS game` |
| `module_dir` | rules: where the code lives | `src/` |
| `docs` | `{{repo_rules}}`: docs each role reads | `["docs/legal.md"]` |
| `verify_command` | `{{repo_rules}}`: check to run before PASS or DONE | `cargo test` |
| `rules` | `{{repo_rules}}`: free-text hard rules | `["No ROM dumps in the repo."]` |

`repo_rules` is built from `docs`, `verify_command` and `rules`. The renderer (#21) is the only place that does this, so each prompt gets the same text.

## Rules

- Missing required fields are an error when the file is parsed (#20). Unknown fields are an error too, so a typo fails loudly.
- The file is read from the target repo, not from the hub's own repo.

## Open

- The plan's question was `.orchestrator/repo.md` or similar. This picks JSON, so a repo's rules must be written as JSON strings, not Markdown. If the rules grow long, a Markdown file for prose can be added later under the same directory.
