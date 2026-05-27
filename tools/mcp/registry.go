package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTools registra todas as 20 tools MCP no servidor. O `repo` é o path
// absoluto do repo alvo, propagado pra cada handler como `Root` das
// Options das funções do scaffold (e como `root` do arch_analyser).
//
// Cada função register<Area> abaixo mora em tool_<area>.go e adiciona uma ou
// mais tools via mcpsdk.AddTool[In, Out]. As assinaturas de In/Out são
// específicas por tool; o schema JSON é inferido automaticamente das tags
// jsonschema:"..." pelo SDK.
func registerTools(s *mcpsdk.Server, repo string) {
	registerDomain(s, repo)
	registerField(s, repo)
	registerDerive(s, repo)
	registerRepository(s, repo)
	registerService(s, repo)
	registerHandler(s, repo)
	registerRoute(s, repo)
	registerProjection(s, repo)
	registerArch(s, repo)
}

// writingAnnotations padroniza os hints MCP para tools que tocam arquivos
// (geração e edição). ReadOnlyHint=false; DestructiveHint=false (o scaffold
// valida antes de sobrescrever); IdempotentHint=false (recriar gera erro de
// duplicate); OpenWorldHint=false (escopo é o repo local).
func writingAnnotations(title string) *mcpsdk.ToolAnnotations {
	f := false
	return &mcpsdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    false,
		DestructiveHint: &f,
		IdempotentHint:  false,
		OpenWorldHint:   &f,
	}
}

// readOnlyAnnotations é o equivalente pra tools que não mutam arquivos
// (arch_analyze e os resources na Fase 4).
func readOnlyAnnotations(title string) *mcpsdk.ToolAnnotations {
	f := false
	return &mcpsdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    true,
		DestructiveHint: &f,
		IdempotentHint:  true,
		OpenWorldHint:   &f,
	}
}

// destructiveAnnotations padroniza os hints MCP para tools que removem
// arquivos/diretórios do disco (os 5 `*_delete`). ReadOnlyHint=false;
// DestructiveHint=true (sinaliza pro cliente MCP exibir confirmação extra);
// IdempotentHint=false (segunda chamada falha porque o alvo já não existe);
// OpenWorldHint=false (escopo é o repo local).
func destructiveAnnotations(title string) *mcpsdk.ToolAnnotations {
	t := true
	f := false
	return &mcpsdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    false,
		DestructiveHint: &t,
		IdempotentHint:  false,
		OpenWorldHint:   &f,
	}
}
