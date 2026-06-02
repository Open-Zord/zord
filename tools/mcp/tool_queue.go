package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type queueCreateInput struct {
	Name   string `json:"name" jsonschema:"nome do entrypoint do worker em lowercase Go ident (ex.: 'worker', 'billing_worker'). NÃO é o nome da fila Redis — esse vem do env OMNIQ_QUEUE em runtime."`
	Driver string `json:"driver,omitempty" jsonschema:"driver da fila; suportado: 'omniq' (default). Gancho pra futuros drivers (sqs, rabbitmq, kafka)."`
	Repo   string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerQueue(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "scaffold_queue_create",
		Description: "Cria o esqueleto de um entrypoint de worker de fila: bootstrap/<name>/ (setup, configs, pkg, repositories, services, worker) + cmd/<name>/main.go. Filosofia 1 entrypoint == 1 fila == 1 worker; nome da fila Redis vem do env OMNIQ_QUEUE em runtime. Hoje só driver 'omniq' (github.com/not-empty/omniq-go) é suportado. Após rodar, o usuário precisa de `go mod tidy` no repo alvo pra trazer omniq-go ao go.sum.",
		Annotations: writingAnnotations("Scaffold: criar queue worker"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in queueCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("queue_create: %w", err)
		}
		created, err := scaffold.QueueCreate(scaffold.QueueCreateOptions{
			Root:   target,
			Name:   in.Name,
			Driver: scaffold.QueueDriver(in.Driver),
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("queue_create: %w", err)
		}
		return nil, CommonOutput{Created: created}, nil
	})
}
