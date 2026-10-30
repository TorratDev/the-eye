# Per-repo config

The hub fills the `{{...}}` placeholders in the role prompts from one file per repo, `repos/<repo>.md`, in the hub's repo. The format is set by `prompts/README.md`. This doc says how `internal/config` reads it.

## Format

```markdown
# rust-mm

repo_name: rust-mm
repo_summary: DS game reimplementation in Rust
verify: cargo test
worktree_root: ../wt

## all
Rules every role follows.

## executor
Rules for executors only.

## resources
| name | lock |
| --- | --- |
```

- **Header**: `key: value` lines before the first `##` heading. Allowed keys: `repo_name`, `repo_summary` (both required), `verify`, `worktree_root`, `shared_files`. Any other key is an error.
- **Sections**: `## all`, `## orchestrator`, `## investigator`, `## executor`, `## reviewer`, `## resources`. Any other name is an error, and so is a second section with the same name.
- `## resources` is for the hub (locks, dispatch). It is never injected into a prompt.

## Placeholders

| Placeholder | Source |
| --- | --- |
| `{{repo_name}}`, `{{repo_summary}}` | header `repo_name`, `repo_summary` |
| `{{repo_rules}}` | `## all`, then the role's own section, joined by a blank line |

`Config.RepoRules(role)` builds `{{repo_rules}}`. The renderer (#21) is the only place that fills placeholders.

## Why this format

It is the format `prompts/README.md` already describes, so the prompts and the config agree. An earlier draft chose JSON in the target repo. That was wrong: it contradicted `prompts/README.md`, which is the spec.
