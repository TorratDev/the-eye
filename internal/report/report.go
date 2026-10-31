// Package report is the report MCP tool: a session states its outcome, the hub stores it,
// and the sub-issue gets a comment.
package report

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/TorratDev/the-eye/internal/hub"
	"github.com/TorratDev/the-eye/internal/session"
)

// Store is the part of the hub store the tool needs.
type Store interface {
	RecordReport(issue int, role, status, summary string) error
	FinishTask(issue int, state string) error
}

// Commenter posts a comment on an issue.
type Commenter interface {
	Comment(issue int, body string) error
}

// Submit checks a report, stores it and comments it on the issue. Nothing is stored or
// posted for a report that fails the checks.
func Submit(st Store, c Commenter, issue int, role, status, summary string) error {
	if issue <= 0 {
		return fmt.Errorf("issue must be a sub-issue number")
	}
	if !slices.Contains([]string{"executor", "reviewer", "investigator", "orchestrator"}, role) {
		return fmt.Errorf("unknown role %q", role)
	}
	if status != "done" && status != "blocked" {
		return fmt.Errorf("status must be done or blocked, not %q", status)
	}
	summary = strings.TrimSpace(summary)
	if summary == "" || strings.ContainsAny(summary, "\r\n") {
		return fmt.Errorf("summary must be one non-empty line")
	}
	if err := st.RecordReport(issue, role, status, summary); err != nil {
		return err
	}
	// an executor's report ends its task, which frees its pane for the next batch
	if role == "executor" {
		if err := st.FinishTask(issue, session.TaskState(strings.ToUpper(status))); err != nil {
			return err
		}
	}
	return c.Comment(issue, fmt.Sprintf("**%s** report from `%s`: %s", status, role, summary))
}

// GhCommenter comments with the gh CLI, which holds the GitHub login.
type GhCommenter struct {
	Repo string // owner/name
}

func (g GhCommenter) Comment(issue int, body string) error {
	out, err := exec.Command("gh", "issue", "comment", fmt.Sprint(issue), "--repo", g.Repo, "--body", body).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh issue comment #%d: %w: %s", issue, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Tool is the report tool definition. The role comes from HUB_ROLE in the session's env.
func Tool() mcp.Tool {
	return mcp.NewTool("report",
		mcp.WithDescription("Report this task's outcome: status done or blocked, and a one-line summary."),
		mcp.WithNumber("issue", mcp.Required(), mcp.Description("the sub-issue number")),
		mcp.WithString("status", mcp.Required(), mcp.Description("done or blocked")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("one line")),
	)
}

// Handler runs the report tool for the session with the given role.
func Handler(st Store, c Commenter, role string) server.ToolHandlerFunc {
	return func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		issue := int(req.GetFloat("issue", 0))
		status, err := req.RequireString("status")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := Submit(st, c, issue, role, status, summary); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("reported"), nil
	}
}

// Server is an MCP server with the report tool, for one session's stdio transport.
func Server(st *hub.Store, c Commenter, role string) *server.MCPServer {
	s := server.NewMCPServer("hub-report", "0.1.0")
	s.AddTool(Tool(), Handler(st, c, role))
	return s
}
