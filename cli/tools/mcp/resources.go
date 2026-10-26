package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerResources registra os 4 resources read-only do servidor MCP:
//
//   - scaffold://docs/readme      (Resource estático)
//   - scaffold://docs/conventions (Resource estático)
//   - scaffold://domains          (Resource estático, retorna JSON)
//   - scaffold://domain/{name}    (ResourceTemplate, retorna source do domain)
//
// O `repo` é propagado pros handlers que precisam ler arquivos do
// repo alvo (readme e domains).
func registerResources(s *mcpsdk.Server, repo string) {
	registerDocsResources(s, repo)
	registerDomainsResources(s, repo)
}
