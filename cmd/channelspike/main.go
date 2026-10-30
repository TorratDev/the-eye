// Command channelspike checks that an mcp-go stdio server advertises experimental.claude/channel
// in its initialize response, set from the AfterInitialize hook. Spike for #36; not part of the hub.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	hooks := &server.Hooks{}
	hooks.AddAfterInitialize(func(_ context.Context, _ any, _ *mcp.InitializeRequest, res *mcp.InitializeResult) {
		if res.Capabilities.Experimental == nil {
			res.Capabilities.Experimental = map[string]any{}
		}
		res.Capabilities.Experimental["claude/channel"] = map[string]any{}
	})
	s := server.NewMCPServer("channel-spike", "0.0.1", server.WithHooks(hooks))
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
