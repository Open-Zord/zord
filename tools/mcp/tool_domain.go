package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

// domainCreateInput é o argumento da tool scaffold_domain_create.
type domainCreateInput struct {
	Name string `json:"name" jsonschema:"nome do domínio em PascalCase (ex.: 'OrgMembership')"`
	Repo string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

// domainDeleteInput é o argumento da tool scaffold_domain_delete.
type domainDeleteInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase a remover"`
	Table  string `json:"table,omitempty" jsonschema:"override do nome da tabela usado pra checar sentinela no HCL (default: snake_case(domain)+'s')"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerDomain(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_domain_create",
		Description: "Gera o arquivo internal/application/domain/<snake>/<snake>.go com a struct Domain mínima (campo ID, métodos Schema()/SoftDelete()).",
		Annotations: writingAnnotations("Scaffold: criar domain"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in domainCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("domain_create: %w", err)
		}
		path, err := scaffold.DomainCreate(in.Name, scaffold.DomainCreateOptions{Root: target})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("domain_create: %w", err)
		}
		return nil, createdOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_domain_delete",
		Description: "Remove internal/application/domain/<snake>/. Operação destrutiva: falha se ainda houver service/repository/handler/route ou bloco HCL referenciando o domínio (lista todas as deps em vez de parar no primeiro erro).",
		Annotations: destructiveAnnotations("Scaffold: remover domain"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in domainDeleteInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("domain_delete: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.DomainDelete(scaffold.DomainDeleteOptions{Root: target, Domain: in.Domain, Table: in.Table})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("domain_delete: %w", err)
		}
		return nil, deletedOutput(path), nil
	})
}
