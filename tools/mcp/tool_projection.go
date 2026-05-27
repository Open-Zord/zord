package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type projectionCreateInput struct {
	Domain         string `json:"domain" jsonschema:"nome do domínio em PascalCase (ex.: 'UsageRecord')"`
	ProjectionName string `json:"projection_name" jsonschema:"nome da projection em PascalCase (ex.: 'ResourceSummary')"`
	Repo           string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type projectionFieldAddInput struct {
	Domain         string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	ProjectionName string `json:"projection_name" jsonschema:"nome da projection em PascalCase"`
	FieldName      string `json:"field_name" jsonschema:"nome do campo em PascalCase"`
	Type           string `json:"type" jsonschema:"expressão Go do tipo (ex.: 'string', 'float64', '*time.Time', '[]ResourceSummary')"`
	TagDB          string `json:"tag_db,omitempty" jsonschema:"override do nome da coluna db (default: snake do campo)"`
	NoDBTag        bool   `json:"no_db_tag,omitempty" jsonschema:"suprime a tag db pra campos de struct composta que não vêm de StructScan"`
	Repo           string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type projectionFieldRemoveInput struct {
	Domain         string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	ProjectionName string `json:"projection_name" jsonschema:"nome da projection em PascalCase"`
	FieldName      string `json:"field_name" jsonschema:"nome do campo em PascalCase"`
	Repo           string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerProjection(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_projection_create",
		Description: "Anexa uma struct projection vazia ao arquivo do domínio (tipo de retorno de query agregada — GROUP BY, SUM, etc).",
		Annotations: writingAnnotations("Scaffold: criar projection"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in projectionCreateInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("projection_create: %w", err)
		}
		path, err := scaffold.ProjectionCreate(scaffold.ProjectionCreateOptions{
			Root:           target,
			Domain:         in.Domain,
			ProjectionName: in.ProjectionName,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("projection_create: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_projection_field_add",
		Description: "Adiciona um campo a uma projection struct (tag json sempre presente; tag db por default, override via tag_db, suprime com no_db_tag).",
		Annotations: writingAnnotations("Scaffold: adicionar field na projection"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in projectionFieldAddInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("projection_field_add: %w", err)
		}
		path, err := scaffold.ProjectionFieldAdd(scaffold.ProjectionFieldAddOptions{
			Root:           target,
			Domain:         in.Domain,
			ProjectionName: in.ProjectionName,
			FieldName:      in.FieldName,
			TypeStr:        in.Type,
			DBTagSet:       in.TagDB != "",
			DBTagValue:     in.TagDB,
			NoDBTag:        in.NoDBTag,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("projection_field_add: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_projection_field_remove",
		Description: "Remove um campo de uma projection struct.",
		Annotations: writingAnnotations("Scaffold: remover field da projection"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in projectionFieldRemoveInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("projection_field_remove: %w", err)
		}
		path, err := scaffold.ProjectionFieldRemove(scaffold.ProjectionFieldRemoveOptions{
			Root:           target,
			Domain:         in.Domain,
			ProjectionName: in.ProjectionName,
			FieldName:      in.FieldName,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("projection_field_remove: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})
}
