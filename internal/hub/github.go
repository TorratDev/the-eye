package hub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

// ClaimLabel is the visible claim marker on a sub-issue.
const ClaimLabel = "hub:claimed"

const subIssuesQuery = `query($owner: String!, $name: String!, $epic: Int!, $cursor: String) {
  repository(owner: $owner, name: $name) {
    issue(number: $epic) {
      subIssues(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          number title state
          labels(first: 100) { pageInfo { hasNextPage } nodes { name } }
          assignees(first: 100) { pageInfo { hasNextPage } nodes { login } }
          blockedBy(first: 100) { pageInfo { hasNextPage endCursor } nodes { number state } }
        }
      }
    }
  }
}`

const blockersQuery = `query($owner: String!, $name: String!, $number: Int!, $cursor: String) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      blockedBy(first: 100, after: $cursor) { pageInfo { hasNextPage endCursor } nodes { number state } }
    }
  }
}`

// Issue is one sub-issue as GitHub reports it.
type Issue struct {
	Number    int
	Title     string
	State     string
	Labels    []string
	Assignees []string
	BlockedBy []Ref
}

// Ref is a blocker: an issue number and its state ("OPEN" or "CLOSED").
type Ref struct {
	Number int
	State  string
}

// Client talks to the GitHub GraphQL API.
type Client struct {
	URL   string
	Token string
	HTTP  *http.Client
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type blockerPage struct {
	PageInfo pageInfo `json:"pageInfo"`
	Nodes    []Ref    `json:"nodes"`
}

type subIssuePage struct {
	PageInfo pageInfo `json:"pageInfo"`
	Nodes    []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Labels struct {
			PageInfo pageInfo `json:"pageInfo"`
			Nodes    []struct {
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"labels"`
		Assignees struct {
			PageInfo pageInfo `json:"pageInfo"`
			Nodes    []struct {
				Login string `json:"login"`
			} `json:"nodes"`
		} `json:"assignees"`
		BlockedBy blockerPage `json:"blockedBy"`
	} `json:"nodes"`
}

// Epic returns every sub-issue of an epic, with all pages of sub-issues and blockers.
func (c *Client) Epic(owner, name string, epic int) ([]Issue, error) {
	var out []Issue
	var cursor any
	for {
		var resp struct {
			Repository struct {
				Issue *struct {
					SubIssues subIssuePage `json:"subIssues"`
				} `json:"issue"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": owner, "name": name, "epic": epic, "cursor": cursor}
		if err := c.do(subIssuesQuery, vars, &resp); err != nil {
			return nil, err
		}
		if resp.Repository.Issue == nil {
			return nil, fmt.Errorf("issue #%d not found in %s/%s", epic, owner, name)
		}
		page := resp.Repository.Issue.SubIssues
		for _, n := range page.Nodes {
			issue := Issue{Number: n.Number, Title: n.Title, State: n.State, BlockedBy: n.BlockedBy.Nodes}
			if n.Labels.PageInfo.HasNextPage || n.Assignees.PageInfo.HasNextPage {
				return nil, fmt.Errorf("#%d has more than %d labels or assignees; refusing a partial read", n.Number, pageSize)
			}
			for _, l := range n.Labels.Nodes {
				issue.Labels = append(issue.Labels, l.Name)
			}
			for _, a := range n.Assignees.Nodes {
				issue.Assignees = append(issue.Assignees, a.Login)
			}
			if n.BlockedBy.PageInfo.HasNextPage {
				rest, err := c.blockersAfter(owner, name, n.Number, n.BlockedBy.PageInfo.EndCursor)
				if err != nil {
					return nil, err
				}
				issue.BlockedBy = append(issue.BlockedBy, rest...)
			}
			out = append(out, issue)
		}
		if !page.PageInfo.HasNextPage {
			return out, nil
		}
		cursor = page.PageInfo.EndCursor
	}
}

// blockersAfter fetches the remaining blockers of one issue, starting after cursor.
func (c *Client) blockersAfter(owner, name string, number int, cursor string) ([]Ref, error) {
	var out []Ref
	for {
		var resp struct {
			Repository struct {
				Issue struct {
					BlockedBy blockerPage `json:"blockedBy"`
				} `json:"issue"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": owner, "name": name, "number": number, "cursor": cursor}
		if err := c.do(blockersQuery, vars, &resp); err != nil {
			return nil, err
		}
		page := resp.Repository.Issue.BlockedBy
		out = append(out, page.Nodes...)
		if !page.PageInfo.HasNextPage {
			return out, nil
		}
		cursor = page.PageInfo.EndCursor
	}
}

const pageSize = 100

func (c *Client) do(query string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+c.Token)
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github: %s: %s", resp.Status, raw)
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("github graphql: %s", env.Errors[0].Message)
	}
	return json.Unmarshal(env.Data, out)
}

// SetClaimLabel adds or removes the claim marker on an issue, through the gh CLI login.
func SetClaimLabel(repo string, number int, on bool) error {
	num := strconv.Itoa(number)
	if !on {
		return run("gh", "issue", "edit", num, "--repo", repo, "--remove-label", ClaimLabel)
	}
	if err := run("gh", "label", "create", ClaimLabel, "--repo", repo, "--force", "--color", "fbca04", "--description", "Claimed by a hub run"); err != nil {
		return err
	}
	return run("gh", "issue", "edit", num, "--repo", repo, "--add-label", ClaimLabel)
}

// GhToken returns the token of the active gh login.
func GhToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, args[0], err, bytes.TrimSpace(out))
	}
	return nil
}
