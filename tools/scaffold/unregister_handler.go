package scaffold

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"golang.org/x/tools/go/ast/astutil"
)

// UnregisterHandlerOptions parametriza UnregisterHandler.
type UnregisterHandlerOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "UsageRecord").
	Domain string
	// Service é o nome do use case em PascalCase (ex.: "Login", "Export").
	Service string
}

// UnregisterHandler remove a ligação no DI feita por `handler register`:
// apaga o ImportSpec do pacote do handler
// (`cmd/http/handlers/<snake_domain>/<snake_service>`) e o ExprStmt da chamada
// `reg.Provide(<alias>.RegistryKey, _)` em `registerHandlers`.
//
// Diferente de UnregisterService, não há detecção dual de formato do import:
// NAVE-73 estabeleceu alias uniforme `<snake_domain sem _><snake_service sem _>handler`
// por construção, então o alias é derivado direto do par (Domain, Service) via
// planHandler. Se o import existir com outro alias (handler legado que nunca
// passou pelo `handler register`), o lookup falha por ausência — comportamento
// correto, handlers legados não são alvo deste comando.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain e Service são PascalCase exportáveis.
//   - `bootstrap/handlers.go` existe e contém `registerHandlers(reg *registry.Registry)`.
//   - O import existe com o alias esperado.
//   - A linha `reg.Provide(<alias>.RegistryKey, _)` existe em registerHandlers.
//
// Não inspeciona `cmd/http/routes/declarable.go` procurando uses do RegistryKey:
// o fluxo natural de desmontagem é `route unregister` → `handler unregister` →
// `service unregister`, e o compile error guia ao próximo passo quando o
// pacote do handler for apagado. Rodar apenas este comando sem seguir a
// sequência deixa o `Resolve[T](reg, <pkg>.RegistryKey)` na route table válido
// em compile time, mas com `panic` em runtime — responsabilidade do dev.
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/handlers.go`).
func UnregisterHandler(opts UnregisterHandlerOptions) (string, error) {
	plan, err := planHandler(RegisterHandlerOptions(opts))
	if err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	src, err := os.ReadFile(plan.absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", plan.relFile, err)
	}
	file, err := parser.ParseFile(fset, plan.absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", plan.relFile, err)
	}

	registerFn, err := findFreeFunc(file, registerHandlersFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}

	imp := findImportSpec(file, plan.importPath)
	if imp == nil {
		return "", fmt.Errorf("em %s: import %q ausente", plan.relFile, plan.importPath)
	}

	provideStmt := findProvideStmt(registerFn, plan.alias)
	if provideStmt == nil {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) ausente", plan.relFile, plan.alias)
	}

	// A partir daqui validações passaram — segue mutação. Alias sempre presente
	// no ImportSpec pelo invariante de NAVE-73 (uniforme); usamos plan.alias
	// direto em DeleteNamedImport pra refletir esse contrato.
	if !astutil.DeleteNamedImport(fset, file, plan.alias, plan.importPath) {
		return "", fmt.Errorf("em %s: falha ao remover import %q", plan.relFile, plan.importPath)
	}
	registerFn.Body.List = removeStmt(registerFn.Body.List, provideStmt)
	collapseTrailingBlank(fset, registerFn.Body)

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}
