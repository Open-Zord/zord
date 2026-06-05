package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

type requestFieldAddInput struct {
	Domain    string `json:"domain" jsonschema:"nome do domínio em PascalCase (ex.: 'OrgMembership')"`
	Verb      string `json:"verb" jsonschema:"verbo do use case em PascalCase (ex.: 'Create', 'List')"`
	FieldName string `json:"field_name" jsonschema:"nome do campo em PascalCase (ex.: 'Email')"`
	Type      string `json:"type" jsonschema:"expressão Go do tipo (ex.: 'string', '*time.Time')"`
	Validate  string `json:"validate,omitempty" jsonschema:"valor da tag validate (ex.: 'required,email'); vazio omite a tag"`
	Repo      string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type requestFieldRemoveInput struct {
	Domain    string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	Verb      string `json:"verb" jsonschema:"verbo do use case em PascalCase"`
	FieldName string `json:"field_name" jsonschema:"nome do campo em PascalCase"`
	Repo      string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerRequest(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_request_field_add",
		Description: "Adiciona um campo à struct Data do request de um use case (sempre com tag json:\"<snake>\"; opcionalmente validate).",
		Annotations: writingAnnotations("Scaffold: adicionar field no request Data"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in requestFieldAddInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("request_field_add: %w", err)
		}
		path, err := scaffold.RequestFieldAdd(scaffold.RequestFieldAddOptions{
			Root:      target,
			Domain:    in.Domain,
			Verb:      in.Verb,
			FieldName: in.FieldName,
			TypeStr:   in.Type,
			Validate:  in.Validate,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("request_field_add: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_request_field_remove",
		Description: "Remove um campo da struct Data do request de um use case. Imports residuais sem uso são removidos.",
		Annotations: writingAnnotations("Scaffold: remover field do request Data"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in requestFieldRemoveInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("request_field_remove: %w", err)
		}
		path, err := scaffold.RequestFieldRemove(scaffold.RequestFieldRemoveOptions{
			Root:      target,
			Domain:    in.Domain,
			Verb:      in.Verb,
			FieldName: in.FieldName,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("request_field_remove: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})
}
