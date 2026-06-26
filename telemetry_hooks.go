package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/telemetry"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newTelemetryHooks wires the telemetry recorder into the MCP server so that
// every tool call (from any client) is observed centrally.
func newTelemetryHooks(rec *telemetry.Recorder) *server.Hooks {
	hooks := &server.Hooks{}

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, msg *mcp.CallToolRequest) {
		rec.Begin(callKey(id))
	})

	hooks.AddAfterCallTool(func(ctx context.Context, id any, msg *mcp.CallToolRequest, result *mcp.CallToolResult) {
		isError := result != nil && result.IsError
		rec.Record(callKey(id), msg.Params.Name, msg.Params.Arguments, resultText(result), isError)
	})

	// OnError covers transport/framework failures that never produce a tool
	// result (e.g. a panic recovered by the server).
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		if method != mcp.MethodToolsCall {
			return
		}
		name := ""
		if req, ok := message.(*mcp.CallToolRequest); ok {
			name = req.Params.Name
		}
		rec.Record(callKey(id), name, nil, fmt.Sprintf("error: %v", err), true)
	})

	return hooks
}

func callKey(id any) string {
	return fmt.Sprintf("%v", id)
}

// resultText concatenates the text content of a tool result for telemetry.
func resultText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range result.Content {
		if tc, ok := mcp.AsTextContent(c); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
