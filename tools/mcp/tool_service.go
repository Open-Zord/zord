package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type serviceCreateInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Verb   string `json:"verb" jsonschema:"verbo do use case em PascalCase (ex.: 'SelectOrg')"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type serviceRegisterInput struct {
	Domain string `json:"domain"`
	Verb   string `json:"verb"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type serviceUnregisterInput struct {
	Domain string `json:"domain"`
	Verb   string `json:"verb"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type serviceDeleteInput struct {
	Domain string `json:"domain"`
	Verb   string `json:"verb"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerService(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_service_create",
		Description: "Gera internal/application/services/<snake_domain>/<snake_verb>/service.go com Service{}, NewService e Execute.",
		Annotations: writingAnnotations("Scaffold: criar service"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in serviceCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_create: %w", err)
		}
		paths, err := scaffold.ServiceCreate(scaffold.ServiceCreateOptions{Root: target, Domain: in.Domain, Verb: in.Verb})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_create: %w", err)
		}
		return nil, CommonOutput{Created: paths}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_service_register",
		Description: "Edita bootstrap/service.go registrando o service no DI container (pkg/registry).",
		Annotations: writingAnnotations("Scaffold: registrar service no DI"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in serviceRegisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_register: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.RegisterService(scaffold.RegisterServiceOptions{Root: target, Domain: in.Domain, Verb: in.Verb})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_register: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_service_unregister",
		Description: "Edita bootstrap/service.go desfazendo o registro do service no DI (inverso de scaffold_service_register).",
		Annotations: writingAnnotations("Scaffold: desregistrar service do DI"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in serviceUnregisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_unregister: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.UnregisterService(scaffold.UnregisterServiceOptions{Root: target, Domain: in.Domain, Verb: in.Verb})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_unregister: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_service_delete",
		Description: "Remove internal/application/services/<snake_domain>/<snake_verb>/. Operação destrutiva: falha se o service ainda estiver registrado no DI (rodar scaffold_service_unregister antes).",
		Annotations: destructiveAnnotations("Scaffold: remover service"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in serviceDeleteInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_delete: %w", err)
		}
		path, err := scaffold.ServiceDelete(scaffold.ServiceDeleteOptions{Root: target, Domain: in.Domain, Verb: in.Verb})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("service_delete: %w", err)
		}
		return nil, deletedOutput(path), nil
	})
}
