package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-Zord/zord/tools/scaffold"
)

// parseFieldTags converte ["db:id", "json:id"] em []scaffold.FieldTag. O
// scaffold espera tags na ordem canônica (FieldCanonicalTagOrder) — mas
// como o cliente MCP é quem decide, deixamos a validação de ordem com o
// próprio scaffold (que falha rápido se as tags estiverem fora de ordem).
func parseFieldTags(raw []string) ([]scaffold.FieldTag, error) {
	out := make([]scaffold.FieldTag, 0, len(raw))
	for _, t := range raw {
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			return nil, fmt.Errorf("tag malformada %q: esperado 'name:value'", t)
		}
		out = append(out, scaffold.FieldTag{Key: k, Value: v})
	}
	return out, nil
}

type fieldAddInput struct {
	Domain    string   `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	FieldName string   `json:"field_name" jsonschema:"nome do campo em PascalCase"`
	Type      string   `json:"type" jsonschema:"expressão Go do tipo (ex.: 'string', '*time.Time')"`
	Tags      []string `json:"tags,omitempty" jsonschema:"tags na ordem canônica (db, json, validate, ...) — formato 'name:value'"`
	Repo      string   `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type fieldRemoveInput struct {
	Domain    string `json:"domain" jsonschema:"nome do domínio em PascalCase"`
	FieldName string `json:"field_name" jsonschema:"nome do campo em PascalCase"`
	Repo      string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type fieldSetTagInput struct {
	Domain    string `json:"domain"`
	FieldName string `json:"field_name"`
	Tag       string `json:"tag" jsonschema:"tag no formato 'name:value'"`
	Repo      string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

type fieldSetTypeInput struct {
	Domain    string `json:"domain"`
	FieldName string `json:"field_name"`
	Type      string `json:"type" jsonschema:"nova expressão Go do tipo"`
	Repo      string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

func registerField(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_field_add",
		Description: "Adiciona um campo à struct de domínio (formato canônico de tags db/json/validate).",
		Annotations: writingAnnotations("Scaffold: adicionar field"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in fieldAddInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("field_add: %w", err)
		}
		tags, err := parseFieldTags(in.Tags)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("field_add: %w", err)
		}
		path, err := scaffold.FieldAdd(scaffold.FieldAddOptions{
			Root:      target,
			Domain:    in.Domain,
			FieldName: in.FieldName,
			TypeStr:   in.Type,
			Tags:      tags,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("field_add: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_field_remove",
		Description: "Remove um campo da struct de domínio.",
		Annotations: writingAnnotations("Scaffold: remover field"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in fieldRemoveInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("field_remove: %w", err)
		}
		path, err := scaffold.FieldRemove(scaffold.FieldRemoveOptions{
			Root:      target,
			Domain:    in.Domain,
			FieldName: in.FieldName,
		})
		if err != nil {
			return nil, CommonOutput{}, fmt.Errorf("field_remove: %w", err)
		}
		return nil, modifiedOutput(path), nil
	})

	// Placeholders — o scaffold não tem essas operações ainda; retorna erro
	// claro pro cliente entender que precisa abrir uma task.
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_field_set_tag",
		Description: "[placeholder] Substitui o valor de uma tag de um campo existente. Não implementado.",
		Annotations: writingAnnotations("Scaffold: set tag (placeholder)"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ fieldSetTagInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		return nil, CommonOutput{}, errors.New("scaffold_field_set_tag não implementado — abrir task NAVE pra adicionar a operação em tools/scaffold")
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scaffold_field_set_type",
		Description: "[placeholder] Substitui o tipo de um campo existente. Não implementado.",
		Annotations: writingAnnotations("Scaffold: set type (placeholder)"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ fieldSetTypeInput) (*mcpsdk.CallToolResult, CommonOutput, error) {
		return nil, CommonOutput{}, errors.New("scaffold_field_set_type não implementado — abrir task NAVE pra adicionar a operação em tools/scaffold")
	})
}
