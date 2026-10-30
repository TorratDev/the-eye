package prompts

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/TorratDev/the-eye/internal/config"
)

var placeholder = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)

// Render replaces each {{name}} in text with vars[name]. It fails on a placeholder with no
// value, so a prompt is never sent with a placeholder left in it.
func Render(text string, vars map[string]string) (string, error) {
	var missing []string
	out := placeholder.ReplaceAllStringFunc(text, func(m string) string {
		name := placeholder.FindStringSubmatch(m)[1]
		v, ok := vars[name]
		if !ok {
			missing = append(missing, name)
			return m
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("unfilled placeholder(s): %s", strings.Join(uniq(missing), ", "))
	}
	return out, nil
}

// RenderRole fills one role's prompt from the repo config: {{repo_name}}, {{repo_summary}}
// and {{repo_rules}} (the all section, then the role's section).
func RenderRole(role string, text string, c config.Config) (string, error) {
	return Render(text, map[string]string{
		"repo_name":    c.Header["repo_name"],
		"repo_summary": c.Header["repo_summary"],
		"repo_rules":   c.RepoRules(role),
	})
}

func uniq(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
