package hub

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// fakeGitHub answers the two GraphQL queries with two pages of sub-issues, and two pages of
// blockers for sub-issue #2.
func fakeGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	const subIssuesPage1 = `{"data":{"repository":{"issue":{"subIssues":{
		"pageInfo":{"hasNextPage":true,"endCursor":"s1"},
		"nodes":[{"number":2,"title":"two","state":"OPEN",
			"labels":{"pageInfo":{"hasNextPage":false},"nodes":[{"name":"hub:claimed"}]},
			"assignees":{"pageInfo":{"hasNextPage":false},"nodes":[{"login":"torrat"}]},
			"blockedBy":{"pageInfo":{"hasNextPage":true,"endCursor":"b1"},"nodes":[{"number":10,"state":"OPEN"}]}}]}}}}}`
	const subIssuesPage2 = `{"data":{"repository":{"issue":{"subIssues":{
		"pageInfo":{"hasNextPage":false,"endCursor":""},
		"nodes":[{"number":3,"title":"three","state":"CLOSED",
			"labels":{"pageInfo":{"hasNextPage":false},"nodes":[]},
			"assignees":{"pageInfo":{"hasNextPage":false},"nodes":[]},
			"blockedBy":{"pageInfo":{"hasNextPage":false},"nodes":[]}}]}}}}}`
	const blockersPage2 = `{"data":{"repository":{"issue":{"blockedBy":{
		"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"number":11,"state":"CLOSED"}]}}}}}`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		if strings.Contains(req.Query, "subIssues") {
			if req.Variables["cursor"] == "s1" {
				io.WriteString(w, subIssuesPage2)
			} else {
				io.WriteString(w, subIssuesPage1)
			}
			return
		}
		if req.Variables["cursor"] != "b1" {
			t.Errorf("blockers asked with cursor %v, want b1", req.Variables["cursor"])
		}
		io.WriteString(w, blockersPage2)
	}))
}

func TestEpicReturnsAllPagesOfSubIssuesAndBlockers(t *testing.T) {
	srv := fakeGitHub(t)
	defer srv.Close()

	c := &Client{URL: srv.URL, Token: "t", HTTP: srv.Client()}
	issues, err := c.Epic("TorratDev", "the-eye", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d sub-issues, want 2", len(issues))
	}
	two := issues[0]
	if two.Number != 2 || two.State != "OPEN" || !slices.Equal(two.Labels, []string{ClaimLabel}) || !slices.Equal(two.Assignees, []string{"torrat"}) {
		t.Fatalf("issue 2 = %+v", two)
	}
	if len(two.BlockedBy) != 2 || two.BlockedBy[0].Number != 10 || two.BlockedBy[1] != (Ref{Number: 11, State: "CLOSED"}) {
		t.Fatalf("blockers of 2 = %+v, want 10 then closed 11", two.BlockedBy)
	}
	if issues[1].Number != 3 || issues[1].State != "CLOSED" {
		t.Fatalf("issue 3 = %+v", issues[1])
	}
}

func TestEpicReportsGraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":null,"errors":[{"message":"Could not resolve to a Repository"}]}`)
	}))
	defer srv.Close()

	c := &Client{URL: srv.URL, Token: "t", HTTP: srv.Client()}
	if _, err := c.Epic("nobody", "missing", 1); err == nil || !strings.Contains(err.Error(), "Could not resolve") {
		t.Fatalf("err = %v, want the GraphQL error", err)
	}
}
