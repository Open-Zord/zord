package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type repositoryCreateInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type repositoryPortInput struct {
	Domain      string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Table       string `json:"table,omitempty" jsonschema:"override do nome da tabela em Schema()"`
	MultiTenant bool   `json:"multi_tenant,omitempty" jsonschema:"ativa o padrão client (campo + setter + prefixo de schema)"`
	Repo        string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type repositoryRegisterInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type repositoryUnregisterInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type repositoryDeleteInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type repositoryUnportInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerRepository(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_repository_create",
		Description: "Gera internal/repositories/<snake>/repository.go com o adapter sqlx que embeda *base_repository.BaseRepo[T].",
		Annotations: writingAnnotations("Scaffold: criar repository"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in repositoryCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_create: %w", err)
		}
		path, err := scaffold.RepositoryCreate(scaffold.RepositoryCreateOptions{Root: target, Domain: in.Domain})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_create: %w", err)
		}
		return nil, createdOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_repository_port",
		Description: "Edita o arquivo do domínio adicionando métodos/interface que satisfazem base_repository.BaseRepository[T].",
		Annotations: writingAnnotations("Scaffold: port do repository no domain"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in repositoryPortInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_port: %w", err)
		}
		path, err := scaffold.RepositoryPort(scaffold.RepositoryPortOptions{
			Root:        target,
			Domain:      in.Domain,
			Table:       in.Table,
			MultiTenant: in.MultiTenant,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_port: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_repository_register",
		Description: "Edita bootstrap/repository.go registrando o repository no DI container (pkg/registry).",
		Annotations: writingAnnotations("Scaffold: registrar repository no DI"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in repositoryRegisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_register: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.RegisterRepository(scaffold.RegisterRepositoryOptions{Root: target, Domain: in.Domain})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_register: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	registerRepositoryInverse(s, repo)
}

func registerRepositoryInverse(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_repository_unregister",
		Description: "Edita bootstrap/repository.go desfazendo o registro do repository no DI (inverso de scaffold_repository_register).",
		Annotations: writingAnnotations("Scaffold: desregistrar repository do DI"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in repositoryUnregisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_unregister: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.UnregisterRepository(scaffold.UnregisterRepositoryOptions{Root: target, Domain: in.Domain})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_unregister: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_repository_delete",
		Description: "Remove internal/repositories/<snake>/. Operação destrutiva: falha se o repository ainda estiver registrado no DI (rodar scaffold_repository_unregister antes).",
		Annotations: destructiveAnnotations("Scaffold: remover repository"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in repositoryDeleteInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_delete: %w", err)
		}
		path, err := scaffold.RepositoryDelete(scaffold.RepositoryDeleteOptions{Root: target, Domain: in.Domain})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_delete: %w", err)
		}
		return nil, deletedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_repository_unport",
		Description: "Edita o arquivo do domínio removendo port + métodos que satisfazem base_repository.BaseRepository[T] (inverso de scaffold_repository_port).",
		Annotations: writingAnnotations("Scaffold: unport do repository no domain"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in repositoryUnportInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_unport: %w", err)
		}
		path, err := scaffold.RepositoryUnport(scaffold.RepositoryUnportOptions{Root: target, Domain: in.Domain})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("repository_unport: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})
}
