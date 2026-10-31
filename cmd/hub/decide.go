package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TorratDev/the-eye/internal/decision"
	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/spawner"
)

// orchestratorReply returns the orchestrator's reply from its Stop event (docs/hooks.md).
func orchestratorReply(e hub.Event) (string, bool) {
	if e.Hook != "Stop" || e.Role != "orchestrator" {
		return "", false
	}
	var p struct {
		Reply string `json:"last_assistant_message"`
	}
	if json.Unmarshal([]byte(e.Payload), &p) != nil || p.Reply == "" {
		return "", false
	}
	return p.Reply, true
}

// actOnStop returns the hook callback that carries out the orchestrator's decisions. Act runs in
// the background: sends wait for idle panes, and the hook must return at once.
func actOnStop(d decision.Deps, done func(decision.Result, error)) func(hub.Event) {
	return func(e hub.Event) {
		if reply, ok := orchestratorReply(e); ok {
			go func() { done(decision.Act(reply, d)) }()
		}
	}
}

// decisionsPane is where the hub shows what it did with the orchestrator's decisions.
const decisionsPane = "decisions"

// showResult prints what was dispatched and puts failures, queued tasks and errors on the page.
func showResult(st *hub.Store, broker *live.Broker) func(decision.Result, error) {
	return func(res decision.Result, err error) {
		if err != nil {
			alert(st, broker, "hub", decisionsPane, "decision lines refused: "+err.Error())
			return
		}
		for _, d := range res.Dispatched {
			fmt.Println("hub:", d)
		}
		for _, q := range res.Queued {
			// shortcut: a queued TASK is only shown, not re-sent; the orchestrator has to send it again
			alert(st, broker, "hub", decisionsPane, "queued past the batch cap: "+q)
		}
		for _, f := range res.Failed {
			alert(st, broker, "hub", decisionsPane, "failed: "+f)
		}
	}
}

// liveDeps are the real effects of a decision for one epic. panes maps pane names to herdr pane IDs
// (spawner.Ensure); prompts holds each role's rendered prompt.
func liveDeps(st *hub.Store, broker *live.Broker, repo, repoDir, worktreeRoot string, epic int, panes, prompts map[string]string) decision.Deps {
	names := map[string]string{} // pane ID -> pane name
	for name, id := range panes {
		names[id] = name
	}
	return decision.Deps{
		Claim: func(n int, owner string) (bool, error) { return hub.Claim(st, repo, epic, n, owner) },
		Worktree: func(n int, slug string) (string, error) {
			if worktreeRoot == "" {
				return "", fmt.Errorf("the repo config has no worktree_root")
			}
			root := worktreeRoot
			if !filepath.IsAbs(root) {
				root = filepath.Join(repoDir, root)
			}
			return spawner.Worktree(repoDir, n, slug, filepath.Join(root, fmt.Sprintf("%d-%s", n, slug)))
		},
		FreeExecutorPane: func() (string, error) {
			for _, r := range spawner.Roles {
				for _, name := range r.Panes {
					if !strings.HasPrefix(name, "executor-") {
						continue
					}
					busy, err := st.PaneBusy(panes[name])
					if err != nil || !busy {
						return panes[name], err
					}
				}
			}
			return "", nil
		},
		InvestigatorPane: func(letter string) (string, error) {
			if id := panes["investigator-"+strings.ToLower(letter)]; id != "" {
				return id, nil
			}
			return "", fmt.Errorf("no investigator pane for %q", letter)
		},
		Send: func(pane, text string) error {
			// sessions start only when a batch needs them (plan: usage limits)
			running, err := agentRunning(pane)
			if err != nil {
				return err
			}
			if !running {
				role, _, _ := strings.Cut(names[pane], "-")
				// a session the hub ran here before (lost to a herdr restart) is resumed, not restarted
				resume, err := st.LastSession(pane)
				if err != nil {
					return err
				}
				if err := spawner.Start(role, pane, prompts[role], resume); err != nil {
					return err
				}
			}
			return spawner.SendWhenIdle(pane, text, 2*time.Minute)
		},
		SetTask: func(n int, pane, worktree string) error {
			return st.SetTask(n, hub.Task{State: "executing", Worktree: worktree, Pane: pane})
		},
		CreateIssue: func(title, body string) (int, error) { return createSubIssue(repo, epic, title, body) },
		// on the orchestrator's own pane, so the page offers its session page to answer on
		Escalate: func(text string) error {
			alert(st, broker, "orchestrator", panes["orchestrator"], "ESCALATE "+text)
			return nil
		},
	}
}

// createSubIssue opens an issue and adds it to the epic, so the hub's sync picks it up.
func createSubIssue(repo string, epic int, title, body string) (int, error) {
	out, err := exec.Command("gh", "issue", "create", "-R", repo, "--title", title, "--body", body).Output()
	if err != nil {
		return 0, fmt.Errorf("gh issue create: %w", err)
	}
	url := strings.TrimSpace(string(out))
	n, err := strconv.Atoi(url[strings.LastIndex(url, "/")+1:])
	if err != nil {
		return 0, fmt.Errorf("gh issue create printed %q", url)
	}
	id, err := exec.Command("gh", "api", fmt.Sprintf("repos/%s/issues/%d", repo, n), "-q", ".id").Output()
	if err != nil {
		return n, fmt.Errorf("created #%d, but could not read its id: %w", n, err)
	}
	link := exec.Command("gh", "api", "-X", "POST", fmt.Sprintf("repos/%s/issues/%d/sub_issues", repo, epic), "-F", "sub_issue_id="+strings.TrimSpace(string(id)))
	if out, err := link.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "hub: %s\n", out)
		return n, fmt.Errorf("created #%d, but could not add it to epic #%d: %w", n, epic, err)
	}
	return n, nil
}
