package config

import "testing"

const sample = `# rust-mm

repo_name: rust-mm
repo_summary: DS game reimplementation in Rust
verify: cargo test
worktree_root: ../wt

## all
No ROM dumps in the repo.

## executor
Edit only the named files.

## resources
| name | lock |
| --- | --- |
`

func TestParseAndRules(t *testing.T) {
	c, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if c.Header["repo_name"] != "rust-mm" || c.Header["verify"] != "cargo test" {
		t.Errorf("header = %v", c.Header)
	}
	if got := c.RepoRules("executor"); got != "No ROM dumps in the repo.\n\nEdit only the named files." {
		t.Errorf("RepoRules(executor) = %q", got)
	}
	if got := c.RepoRules("reviewer"); got != "No ROM dumps in the repo." {
		t.Errorf("RepoRules(reviewer) = %q", got)
	}
}

func TestParseRefusesTypos(t *testing.T) {
	cases := map[string]string{
		"missing name":    "repo_summary: x\n",
		"unknown key":     "repo_name: a\nrepo_summary: b\nverfy: cargo\n",
		"unknown section": "repo_name: a\nrepo_summary: b\n## exectuor\nx\n",
		"duplicate":       "repo_name: a\nrepo_summary: b\n## all\nx\n## all\ny\n",
	}
	for name, text := range cases {
		if _, err := Parse(text); err == nil {
			t.Errorf("%s: accepted, want error", name)
		}
	}
}
