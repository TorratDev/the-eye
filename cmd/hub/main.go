package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/TorratDev/the-eye/internal/cloud"
	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/report"
	"github.com/TorratDev/the-eye/internal/ui"
	"github.com/mark3labs/mcp-go/server"
)

const (
	usage     = "usage: hub sync [--repo owner/name] <epic>"
	stateFile = ".hub/state.db" // gitignored
	graphQL   = "https://api.github.com/graphql"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hub:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "serve" {
		return serve(args[1:])
	}
	if len(args) > 0 && args[0] == "mcp" {
		return mcpServe(args[1:])
	}
	if len(args) == 0 || args[0] != "sync" {
		return errors.New(usage)
	}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	repo := fs.String("repo", "", "owner/name (default: origin remote)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(usage)
	}
	epic, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("epic must be an issue number: %w", err)
	}
	if *repo == "" {
		if *repo, err = originRepo(); err != nil {
			return err
		}
	}
	owner, name, err := parseRepo(*repo)
	if err != nil {
		return err
	}
	return sync(owner, name, epic)
}

// mcpServe runs the report tool over stdio for one session. It does not open the store: `hub serve`
// holds it, so the session sends its report to the server's POST /report instead.
func mcpServe(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	url := fs.String("url", "http://127.0.0.1:8080", "the hub serve address")
	role := fs.String("role", os.Getenv("HUB_ROLE"), "this session's role (default: HUB_ROLE)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *role == "" {
		return errors.New("mcp needs --role or HUB_ROLE")
	}
	return server.ServeStdio(report.RemoteServer(*url, *role))
}

// serve runs the watcher page, the hook endpoint and the live stream on addr.
func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address; keep it on localhost")
	epic := fs.Int("epic", 0, "epic whose cloud reports to poll from GitHub (0: no polling)")
	poll := fs.Duration("poll", 30*time.Second, "how often to poll cloud reports")
	repoFlag := fs.String("repo", "", "owner/name (default: origin remote)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := hub.OpenStore(stateFile)
	if err != nil {
		return err
	}
	defer st.Close()
	// without a repo the report endpoint refuses, and polling cannot start
	repo := *repoFlag
	if repo == "" {
		repo, _ = originRepo()
	}
	var commenter report.Commenter
	if repo != "" {
		commenter = report.GhCommenter{Repo: repo}
	}
	broker := live.NewBroker()
	if *epic > 0 {
		if repo == "" {
			return errors.New("polling needs --repo or an origin remote")
		}
		owner, name, err := parseRepo(repo)
		if err != nil {
			return err
		}
		go pollCloud(st, broker, owner+"/"+name, owner, name, *epic, *poll)
	}
	fmt.Printf("hub: serving on http://%s (state in %s)\n", *addr, stateFile)
	return http.ListenAndServe(*addr, ui.Handler(st, broker, commenter))
}

// pollCloud reads the epic's open sub-issues every interval and applies new cloud reports.
// Outcomes the orchestrator or the operator must see are recorded as events and published.
func pollCloud(st *hub.Store, broker *live.Broker, repo, owner, name string, epic int, every time.Duration) {
	token, err := hub.GhToken()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hub: cloud poll disabled:", err)
		return
	}
	client := &hub.Client{URL: graphQL, Token: token}
	p := &cloud.Poller{
		Store: st,
		Repo:  repo,
		Fetch: cloud.FetchComments,
		Issues: func() ([]int, error) {
			issues, err := client.Epic(owner, name, epic)
			if err != nil {
				return nil, err
			}
			var open []int
			for _, i := range issues {
				if i.State == "OPEN" {
					open = append(open, i.Number)
				}
			}
			return open, nil
		},
		OnOutcome: func(issue int, out cloud.Outcome) {
			for _, text := range out.Routed {
				recordAlert(st, broker, issue, "cloud BLOCKED", text)
			}
			for _, text := range out.Refused {
				recordAlert(st, broker, issue, "cloud report refused", text)
			}
		},
		OnError: func(issue int, err error) {
			fmt.Fprintf(os.Stderr, "hub: cloud poll #%d: %v\n", issue, err)
		},
	}
	if err := p.Run(context.Background(), every); err != nil {
		fmt.Fprintln(os.Stderr, "hub: cloud poll stopped:", err)
	}
}

// recordAlert stores a cloud finding as a prompt-style event, so the errors panel shows it, and publishes the page.
func recordAlert(st *hub.Store, broker *live.Broker, issue int, what, text string) {
	payload, _ := json.Marshal(map[string]string{"message": fmt.Sprintf("#%d %s: %s", issue, what, text)})
	err := st.RecordEvent(hub.Event{ReceivedAt: time.Now(), Hook: "Notification", Role: "cloud", Pane: fmt.Sprintf("cloud-%d", issue), Payload: string(payload)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hub: record cloud finding:", err)
		return
	}
	if frag, err := ui.SessionsFragment(st); err == nil {
		broker.Publish("state", frag)
	}
}

// sync prints every sub-issue with its state, blockers and ready flag. It dispatches nothing.
func sync(owner, name string, epic int) error {
	token, err := hub.GhToken()
	if err != nil {
		return err
	}
	issues, err := (&hub.Client{URL: graphQL, Token: token}).Epic(owner, name, epic)
	if err != nil {
		return err
	}
	st, err := hub.OpenStore(stateFile)
	if err != nil {
		return err
	}
	defer st.Close()
	claims, err := st.Claims()
	if err != nil {
		return err
	}
	verdicts, cycles := hub.Plan(issues, claims)

	ready := 0
	for _, v := range verdicts {
		if v.Ready {
			ready++
		}
	}
	fmt.Printf("epic #%d in %s/%s: %d sub-issues, %d ready\n", epic, owner, name, len(issues), ready)
	for _, v := range verdicts {
		fmt.Printf("#%-4d %-6s ready=%-5t %s %s\n", v.Issue.Number, v.Issue.State, v.Ready, detail(v, claims), v.Issue.Title)
	}
	for _, c := range cycles {
		fmt.Fprintf(os.Stderr, "cycle: %s\n", cycleText(c))
	}
	if len(cycles) > 0 {
		return fmt.Errorf("%d dependency cycle(s); nothing on them is ready", len(cycles))
	}
	return nil
}

// detail says why a sub-issue is or is not ready.
func detail(v hub.Verdict, claims map[int]string) string {
	switch {
	case v.Issue.State != "OPEN":
		return "closed"
	case v.Cycle:
		return "in cycle"
	case v.Claimed && claims[v.Issue.Number] != "":
		return "claimed by " + claims[v.Issue.Number]
	case v.Claimed:
		return "claimed on GitHub"
	case len(v.Waiting) > 0:
		return "waiting on " + refList(v.Waiting)
	default:
		return "-"
	}
}

func refList(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("#%d", n)
	}
	return strings.Join(parts, ", ")
}

func cycleText(nums []int) string {
	return refList(append(nums, nums[0])) // close the loop: #a, #b, #a
}

// originRepo reads owner/name from the origin remote of the current directory.
func originRepo() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("no --repo given and no git origin remote: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parseRepo accepts owner/name, an https URL or an ssh remote, with or without .git.
func parseRepo(s string) (owner, name string, err error) {
	parts := strings.FieldsFunc(strings.TrimSuffix(strings.TrimSpace(s), ".git"), func(r rune) bool {
		return r == '/' || r == ':'
	})
	if len(parts) < 2 {
		return "", "", fmt.Errorf("cannot read owner/name from %q", s)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}
