package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type routeCreateInput struct {
	Domain string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type routeAddInput struct {
	Domain  string `json:"domain"`
	Service string `json:"service" jsonschema:"nome do use case em PascalCase"`
	Method  string `json:"method" jsonschema:"verbo HTTP (GET|POST|PUT|PATCH|DELETE), case-insensitive"`
	Path    string `json:"path,omitempty" jsonschema:"override do path default (kebab-case do service)"`
	Public  bool   `json:"public,omitempty" jsonschema:"registra em DeclarePublicRoutes ao invés de DeclarePrivateRoutes"`
	Repo    string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type routeRegisterInput struct {
	Domain string `json:"domain"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type routeUnregisterInput struct {
	Domain string `json:"domain"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type routeRemoveInput struct {
	Domain  string `json:"domain"`
	Service string `json:"service" jsonschema:"nome do use case em PascalCase"`
	Force   bool   `json:"force,omitempty" jsonschema:"remove só os pontos AST que existirem em vez de exigir os 4 (útil pra retry após falha parcial)"`
	Repo    string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type routeDeleteInput struct {
	Domain string `json:"domain"`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerRoute(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_route_create",
		Description: "Gera cmd/http/routes/<snake>.go com struct <Pascal>Route, constructor recebendo *registry.Registry e DeclarePrivate/PublicRoutes vazios.",
		Annotations: writingAnnotations("Scaffold: criar route"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in routeCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_create: %w", err)
		}
		path, err := scaffold.RouteCreate(scaffold.RouteCreateOptions{Root: target, Domain: in.Domain})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_create: %w", err)
		}
		return nil, createdOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_route_add",
		Description: "Edita cmd/http/routes/<snake>.go anexando o campo do handler, atribuição via registry.Resolve e linha de registro em Declare{Private,Public}Routes.",
		Annotations: writingAnnotations("Scaffold: adicionar route HTTP"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in routeAddInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_add: %w", err)
		}
		path, err := scaffold.RouteAdd(scaffold.RouteAddOptions{
			Root:    target,
			Domain:  in.Domain,
			Service: in.Service,
			Method:  in.Method,
			Path:    in.Path,
			Public:  in.Public,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_add: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_route_register",
		Description: "Edita o map central GetRoutes em cmd/http/routes/declarable.go registrando o constructor New<Pascal>Route.",
		Annotations: writingAnnotations("Scaffold: registrar route no map"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in routeRegisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_register: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.RouteRegister(scaffold.RouteRegisterOptions{Root: target, Domain: in.Domain})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_register: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	registerRouteInverse(s, repo)
}

func registerRouteInverse(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_route_unregister",
		Description: "Edita o map central GetRoutes em cmd/http/routes/declarable.go desfazendo o registro do constructor (inverso de scaffold_route_register).",
		Annotations: writingAnnotations("Scaffold: desregistrar route do map"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in routeUnregisterInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_unregister: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.RouteUnregister(scaffold.RouteUnregisterOptions{Root: target, Domain: in.Domain})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_unregister: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_route_remove",
		Description: "Edita cmd/http/routes/<snake>.go desfazendo o route_add do par (Domain, Service): remove campo da struct, KV no ctor, stmt em Declare{Private,Public}Routes e import (se órfão).",
		Annotations: writingAnnotations("Scaffold: remover route HTTP"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in routeRemoveInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_remove: %w", err)
		}
		path, err := scaffold.RouteRemove(scaffold.RouteRemoveOptions{
			Root:    target,
			Domain:  in.Domain,
			Service: in.Service,
			Force:   in.Force,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_remove: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_route_delete",
		Description: "Remove cmd/http/routes/<snake>.go. Operação destrutiva: falha se a route ainda estiver registrada em GetRoutes (rodar scaffold_route_unregister antes).",
		Annotations: destructiveAnnotations("Scaffold: remover route"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in routeDeleteInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_delete: %w", err)
		}
		path, err := scaffold.RouteDelete(scaffold.RouteDeleteOptions{Root: target, Domain: in.Domain})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("route_delete: %w", err)
		}
		return nil, deletedOutput(path), nil
	})
}
