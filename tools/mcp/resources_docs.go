package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// conventionsText resume as convenções que governam o scaffold
// (extraído dos comentários de package em tools/scaffold/*).
// É string estática para o cliente conseguir ler sem depender de arquivos
// específicos do repo.
const conventionsText = `# Convenções do scaffold

## Identificadores
- ULIDs como ID opaco do domínio (pkg/idCreator). Nunca UUID nem autoincrement.
- Nomes de domain em PascalCase (ex.: "OrgMembership", "UsageRecord").
- Nomes de verb de service em PascalCase (ex.: "Login", "SelectOrg").
- Pacotes em snake_case (ex.: package usage_record).

## Geração de código Go
- Sempre via AST puro (go/ast + tools/scaffold/astbuild.go).
- Nunca via template de string ou text/template.
- Toda escrita é localizada: o scaffold falha rápido se o arquivo já existe
  (domain_create, repository_create, service_create, handler_create,
  route_create) ou se o item conflita (field_add com nome duplicado).

## Schema/HCL
- Bloco gerado entre sentinelas:
    # scaffold:generated <table>
    table "..." { ... }
    # scaffold:end <table>
- derive_schema regenera o bloco entre sentinelas; preserva o resto do HCL.
- Tabelas sem sentinela são consideradas manuais — o scaffold se recusa a
  sobrescrever.

## Dependency injection
- pkg/registry é o container DI (chave string → *T).
- bootstrap/ wireia services, repositories e handlers no startup.
- Constructors eager: New<Pascal>Handler chama registry.Resolve no boot, não
  no Handle. Falha rápida.

## Camadas (validadas por tools/arch_analyser)
- cmd/http: handlers + routes (não importa internal/repositories direto).
- internal/application/services: usecase-per-folder; orquestra repositórios.
- internal/application/domain: structs + ports (interfaces).
- internal/repositories: implementação sqlx; embeda *base_repository.BaseRepo[T].
- pkg/: utilitários genéricos sem dependência de internal/.

## Rotas
- 1 use case = 1 service + 1 handler + 1 entrada em route_add.
- Route.constructor recebe apenas *registry.Registry (assinatura imutável).
- DeclarePrivateRoutes vs DeclarePublicRoutes: público se a rota não exige
  JWT user-scope ou org-scope.
`

func registerDocsResources(s *mcpsdk.Server, repo string) {
	s.AddResource(&mcpsdk.Resource{
		Name:        "scaffold_readme",
		Title:       "Scaffold: README",
		URI:         "scaffold://docs/readme",
		Description: "README do pacote tools/scaffold (overview das operações, convenções e exemplos).",
		MIMEType:    "text/markdown",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		readmePath := filepath.Join(repo, "tools/scaffold/README.md")
		body, err := os.ReadFile(readmePath) //nolint:gosec // G304: path resolvido a partir do repo controlado
		if err != nil {
			return nil, fmt.Errorf("ler scaffold README %q: %w", readmePath, err)
		}
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     string(body),
			}},
		}, nil
	})

	s.AddResource(&mcpsdk.Resource{
		Name:        "scaffold_conventions",
		Title:       "Scaffold: convenções",
		URI:         "scaffold://docs/conventions",
		Description: "Convenções aplicáveis ao scaffold (IDs, geração AST, schema HCL, DI, camadas).",
		MIMEType:    "text/markdown",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     conventionsText,
			}},
		}, nil
	})
}
