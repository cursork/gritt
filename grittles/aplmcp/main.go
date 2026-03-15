// aplmcp is an MCP server for LLM ↔ Dyalog APL interaction.
// Reads JSON-RPC 2.0 from stdin, writes responses to stdout.
//
// Claude Desktop config:
//
//	{ "mcpServers": { "apl": { "command": "aplmcp" } } }
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cursork/gritt/mcp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := mcp.NewServer()
	if err := srv.Serve(ctx, os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
