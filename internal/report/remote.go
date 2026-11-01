package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Remote is the report tool for a session whose MCP process cannot open the hub's store (the store
// is locked while `hub serve` runs). It sends the report to the hub's POST /report, which stores and
// comments it. The role is the session's own, from HUB_ROLE, not from the tool caller.
func Remote(hubURL, role string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := req.RequireString("status")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, err := json.Marshal(map[string]any{
			"issue":   int(req.GetFloat("issue", 0)),
			"status":  status,
			"summary": summary,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, hubURL+"/report", bytes.NewReader(body))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-Hub-Role", role)
		res, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("hub unreachable: %v", err)), nil
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			// the hub writes why it refused (report.Submit's error), so the session can correct its call
			reason, _ := io.ReadAll(io.LimitReader(res.Body, 1<<10))
			msg := "hub refused the report: " + res.Status
			if r := strings.TrimSpace(string(reason)); r != "" {
				msg += ": " + r
			}
			return mcp.NewToolResultError(msg), nil
		}
		return mcp.NewToolResultText("reported"), nil
	}
}

// RemoteServer is the stdio MCP server a session runs for the report tool.
func RemoteServer(hubURL, role string) *server.MCPServer {
	s := server.NewMCPServer("hub-report", "0.1.0")
	s.AddTool(Tool(), Remote(hubURL, role))
	return s
}
