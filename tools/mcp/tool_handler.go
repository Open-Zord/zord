package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type handlerCreateInput struct {
	Domain  string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Service string `json:"service" jsonschema:"nome do use case em PascalCase (ex.: 'Login')"`
	Repo    string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type handlerRegisterInput struct {
	Domain  string `json:"domain"`
	Service string `json:"service"`
	Repo    string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type handlerUnregisterInput struct {
	Domain  string `json:"domain"`
	Service string `json:"service"`
	Repo    string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type handlerDeleteInput struct {
	Domain  string `json:"domain"`
	Service string `json:"service"`
	Repo    string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerHandler(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_handler_create",
		Description: "Gera cmd/http/handlers/<snake_domain>/<snake_service>/handler.go com <Pascal>Handler, New<Pascal>Handler (eager registry.Resolve) e Handle.",
		Annotations: writingAnnotations("Scaffold: criar handler"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in handlerCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_create: %w", err)
		}
		path, err := scaffold.HandlerCreate(scaffold.HandlerCreateOptions{Root: target, Domain: in.Domain, Service: in.Service})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_create: %w", err)
		}
		return nil, createdOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_handler_register",
		Description: "Edita bootstrap/handler.go registrando o handler no DI container.",
		Annotations: writingAnnotations("Scaffold: registrar handler no DI"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in handlerRegisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_register: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.RegisterHandler(scaffold.RegisterHandlerOptions{Root: target, Domain: in.Domain, Service: in.Service})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_register: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_handler_unregister",
		Description: "Edita bootstrap/handler.go desfazendo o registro do handler no DI (inverso de scaffold_handler_register).",
		Annotations: writingAnnotations("Scaffold: desregistrar handler do DI"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in handlerUnregisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_unregister: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.UnregisterHandler(scaffold.UnregisterHandlerOptions{Root: target, Domain: in.Domain, Service: in.Service})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_unregister: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_handler_delete",
		Description: "Remove cmd/http/handlers/<snake_domain>/<snake_service>/. Operação destrutiva: falha se o handler ainda estiver registrado no DI (rodar scaffold_handler_unregister antes).",
		Annotations: destructiveAnnotations("Scaffold: remover handler"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in handlerDeleteInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_delete: %w", err)
		}
		path, err := scaffold.HandlerDelete(scaffold.HandlerDeleteOptions{Root: target, Domain: in.Domain, Service: in.Service})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("handler_delete: %w", err)
		}
		return nil, deletedOutput(path), nil
	})
}
