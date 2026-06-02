package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type entrypointCreateInput struct {
	Name string `json:"name" jsonschema:"nome do entrypoint em lowercase Go ident (ex.: 'cli', 'grpc', 'redis_queue')"`
	Repo string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerEntrypoint(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_entrypoint_create",
		Description: "Cria o esqueleto mínimo de um novo entrypoint do monorepo: bootstrap/<name>/setup.go (registry vazio) e cmd/<name>/main.go (chama Setup e descarta o registry). Ponto de partida pro dev implementar o runtime; futuros scaffolds por tipo (grpc, queue, etc.) vão preencher cada entrypoint com suas próprias camadas.",
		Annotations: writingAnnotations("Scaffold: criar entrypoint"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in entrypointCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("entrypoint_create: %w", err)
		}
		bp, cp, err := scaffold.EntrypointCreate(scaffold.EntrypointCreateOptions{Root: target, Name: in.Name})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("entrypoint_create: %w", err)
		}
		return nil, CommonOutput{Created: []string{bp, cp}}, nil
	})
}
