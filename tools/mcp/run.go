package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run sobe o servidor MCP via stdio. Encerra quando ctx é cancelado (signal)
// ou quando o cliente fecha stdin (EOF detectado pelo transport).
//
// args é equivalente a os.Args[1:] — Run faz o parse de --repo.
func Run(ctx context.Context, args []string) error {
	repo, err := resolveRepo(args)
	if err != nil {
		return err
	}

	log := stderrLogger()
	log.Info("mcp server starting", "name", serverName, "version", serverVersion, "repo", repo)

	server := newServer(repo)
	session, err := server.Connect(ctx, &mcpsdk.StdioTransport{}, nil)
	if err != nil {
		return fmt.Errorf("connect stdio transport: %w", err)
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("session: %w", err)
	}
	log.Info("mcp server stopped")
	return nil
}
