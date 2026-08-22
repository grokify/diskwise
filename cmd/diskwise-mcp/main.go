// Command diskwise-mcp is the DiskWise MCP server: it exposes the same
// read-only storage-discovery queries as the diskwise CLI to MCP
// clients over stdio. Tools are registered as the service layer is
// implemented (see docs/specs/PLAN.md, Phase 5).
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "diskwise-mcp", Version: version}, nil)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("diskwise-mcp: %v", err)
	}
}
