// Package mcp implementa o servidor MCP — expõe as operações
// de tools/scaffold/ (geração de domain, field, repository, service, handler,
// route, schema) e tools/arch_analyser/ como tools MCP, mais resources de
// leitura sobre o repositório alvo. Transport stdio único nesta fatia.
//
// Entrypoint: cmd/mcp/main.go chama mcp.Run(ctx, args).
package mcp

import (
	"log/slog"
	"os"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "zord-mcp"
	serverVersion = "0.1.0"
)

// newServer cria o server MCP com as 20 tools (19 wrappers do scaffold +
// arch_analyze) e os 4 resources read-only registrados.
func newServer(repo string) *mcpsdk.Server {
	s := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: serverName, Version: serverVersion},
		nil,
	)
	registerTools(s, repo)
	registerResources(s, repo)
	return s
}

// stderrLogger devolve um logger estruturado em stderr. Stdout é reservado
// pro protocolo MCP — qualquer escrita lá quebra o JSON-RPC framing.
func stderrLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
