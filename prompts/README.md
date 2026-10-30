# Role prompts

The four role prompts are generic. Before launch, the hub fills in their placeholders from the per-repo config in `repos/<repo>.md`:

| Placeholder | Source in the config |
| --- | --- |
| `{{repo_name}}`, `{{repo_summary}}` | `key: value` lines at the top of the config |
| `{{repo_rules}}` | the `## all` section, then the role's own section (`## executor`, for example) |

The other header keys (`verify`, `worktree_root`, `shared_files`) and the `## Resources` table are for the hub. It uses them for dispatch defaults, file-overlap checks and resource locks. They are not injected.

Values that differ per task (letter, sub-issue, worktree, files, resources) go in the dispatch message, never in the prompt.
