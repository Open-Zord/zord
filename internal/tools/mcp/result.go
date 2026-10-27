package mcp

// CommonOutput é o formato de saída estruturado das tools que mutam arquivos
// (todas exceto arch_analyze). Os campos descrevem o resultado da operação
// pro cliente MCP de forma uniforme:
//
//   - Created: paths relativos dos arquivos criados (vazio se a tool só edita).
//   - Modified: paths relativos dos arquivos editados.
//   - Deleted: paths relativos dos arquivos/diretórios removidos (tools
//     destrutivas: *_delete).
//   - Diff: unified diff opcional (não preenchido na Fase 2; ver Fase 6).
//   - Warnings: avisos não-fatais (ex.: campo já existia, registro idempotente).
//
// Erros fatais não viram CommonOutput — vão pelo retorno `error` do handler,
// que o SDK converte em CallToolResult com IsError: true.
type CommonOutput struct {
	Created  []string `json:"created"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted,omitempty"`
	Diff     string   `json:"diff,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// modifiedOutput é o atalho usado pelos handlers das tools que editam um único
// arquivo (o caso comum: o scaffold devolve o path relativo do arquivo
// alterado e nada mais).
func modifiedOutput(path string) CommonOutput {
	return CommonOutput{Modified: []string{path}}
}

// createdOutput é o atalho pros handlers das tools que criam um arquivo novo
// (domain_create, repository_create, service_create, handler_create,
// route_create, derive_schema na primeira execução).
func createdOutput(path string) CommonOutput {
	return CommonOutput{Created: []string{path}}
}

// deletedOutput é o atalho pros handlers das tools destrutivas que removem
// um arquivo ou diretório do disco (domain_delete, repository_delete,
// service_delete, handler_delete, route_delete).
func deletedOutput(path string) CommonOutput {
	return CommonOutput{Deleted: []string{path}}
}
