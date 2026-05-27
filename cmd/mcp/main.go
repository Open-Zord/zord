// Command mcp é o servidor Model Context Protocol. Expõe
// tools/scaffold/ e tools/arch_analyser/ como tools MCP via transport stdio.
// Doc completa: ver tools/mcp/README.md.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Open-Zord/zord/tools/mcp"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := mcp.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mcp:", err)
		return 1
	}
	return 0
}
