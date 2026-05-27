package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type deriveSchemaInput struct {
	Domain     string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Table      string `json:"table,omitempty" jsonschema:"override do nome da tabela (default: snake_case(domain)+'s')"`
	SchemaPath string `json:"schema_path,omitempty" jsonschema:"override do caminho do arquivo HCL"`
	SchemaName string `json:"schema_name,omitempty" jsonschema:"override do schema referenciado no HCL"`
	Repo       string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type deriveSchemaRemoveInput struct {
	Domain     string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Table      string `json:"table,omitempty" jsonschema:"override do nome da tabela (default: snake_case(domain)+'s')"`
	SchemaPath string `json:"schema_path,omitempty" jsonschema:"override do caminho do arquivo HCL"`
	Repo       string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerDerive(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_derive_schema",
		Description: "Deriva o bloco HCL Atlas da tabela a partir do domínio Go; reexecuta substituindo o bloco entre sentinelas '# scaffold:generated <table>' / '# scaffold:end <table>'.",
		Annotations: writingAnnotations("Scaffold: derivar schema HCL"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in deriveSchemaInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("derive_schema: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.SchemaDerive(scaffold.SchemaDeriveOptions{
				Root:       target,
				Domain:     in.Domain,
				Table:      in.Table,
				SchemaPath: in.SchemaPath,
				SchemaName: in.SchemaName,
			})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("derive_schema: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_derive_schema_remove",
		Description: "Remove o bloco HCL Atlas da tabela do domínio, apagando o conteúdo delimitado pelas sentinelas '# scaffold:generated <table>' / '# scaffold:end <table>'. Falha se a sentinela estiver ausente.",
		Annotations: writingAnnotations("Scaffold: remover schema HCL"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in deriveSchemaRemoveInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("derive_schema_remove: %w", err)
		}
		var path string
		err = withRepoLock(target, func() error {
			var inner error
			path, inner = scaffold.SchemaUnderive(scaffold.SchemaUndriveOptions{
				Root:       target,
				Domain:     in.Domain,
				Table:      in.Table,
				SchemaPath: in.SchemaPath,
			})
			return inner
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("derive_schema_remove: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})
}
