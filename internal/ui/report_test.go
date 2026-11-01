package ui

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/live"
	"github.com/TorratDev/the-eye/internal/report"
)

type fakeComments struct{ n int }

func (f *fakeComments) Comment(issue int, body string) error { f.n++; return nil }

// A report call through the session's MCP server creates one stored record and one issue comment,
// while the hub store is in use by `hub serve`.
func TestReportThroughSessionMCP(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &fakeComments{}
	srv := httptest.NewServer(Handler(st, live.NewBroker(), c, nil, Actions{}))
	defer srv.Close()

	handler := report.Remote(srv.URL, "executor")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"issue": 22.0, "status": "done", "summary": "tests pass"}
	res, err := handler(context.Background(), req)
	if err != nil || res.IsError {
		t.Fatalf("report call failed: %v, %+v", err, res)
	}
	if n, _ := st.Reports(22); n != 1 {
		t.Errorf("stored reports = %d, want 1", n)
	}
	if c.n != 1 {
		t.Errorf("comments = %d, want 1", c.n)
	}
}

// A report the store or GitHub cannot take is refused to the session, not dropped silently.
func TestReportWithoutCommenterRefused(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := httptest.NewServer(Handler(st, live.NewBroker(), nil, nil, Actions{}))
	defer srv.Close()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"issue": 22.0, "status": "done", "summary": "x"}
	res, err := report.Remote(srv.URL, "executor")(context.Background(), req)
	if err != nil || !res.IsError {
		t.Errorf("report with no commenter: res.IsError = %v, err = %v; want refused", res != nil && res.IsError, err)
	}
}

// A refused report tells the session why, so it can correct its call (#168).
func TestRefusedReportNamesReason(t *testing.T) {
	st, err := hub.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := httptest.NewServer(Handler(st, live.NewBroker(), &fakeComments{}, nil, Actions{}))
	defer srv.Close()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"issue": 22.0, "status": "done", "summary": "looks fine"}
	res, err := report.Remote(srv.URL, "reviewer")(context.Background(), req)
	if err != nil || !res.IsError {
		t.Fatalf("a reviewer's done was accepted: %+v, %v", res, err)
	}
	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "400 Bad Request") || !strings.Contains(text, "pass, revise, split, escalate") {
		t.Errorf("refusal = %q, want the status and the hub's reason", text)
	}
}
