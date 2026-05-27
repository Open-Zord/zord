// Package scaffold (área unregister) reverte os wire-ups feitos pelos comandos
// `*** register`. Cada kind tem um arquivo próprio (`unregister_service.go`,
// futuramente `unregister_repository.go`, etc.) e expõe uma função única que
// recebe Options e retorna o caminho relativo do arquivo editado.
//
// `service unregister` (NAVE-88) é simétrico a `service register` (NAVE-60):
// remove o ImportSpec do pacote do verbo e o ExprStmt
// `reg.Provide(<pkg>.RegistryKey, _)` associado em `registerServices`,
// preservando o resto do arquivo intacto.
package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"
)

// UnregisterServiceOptions parametriza UnregisterService.
type UnregisterServiceOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "OrgMembership").
	Domain string
	// Verb é o nome do verbo do use case em PascalCase (ex.: "Login",
	// "SelectOrg").
	Verb string
}

// UnregisterService remove a ligação no DI feita por `service register`:
// apaga o ImportSpec do pacote do verbo (`internal/application/services/
// <snake_domain>/<snake_verb>`) e o ExprStmt da chamada
// `reg.Provide(<pkgIdent>.RegistryKey, _)` em `registerServices`.
//
// Detecta o formato do import:
//   - Bare (sem alias)        → pkgIdent = <snake_verb>
//   - Aliased (com .Name set) → pkgIdent = alias (tipicamente
//     <snake_domain>_<snake_verb>, como `service register` aplica em colisão)
//
// A mesma pkgIdent é usada pra localizar a chamada de Provide. O segundo
// argumento é ignorado: devs evoluem `NewService(log, idC)` adicionando ports
// do domínio, e o unregister precisa funcionar após essa evolução.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain e Verb são PascalCase exportáveis.
//   - `bootstrap/services.go` existe e contém `registerServices(reg *registry.Registry)`.
//   - O import existe (com ou sem alias).
//   - A linha `reg.Provide(<pkgIdent>.RegistryKey, _)` existe em registerServices.
//
// Não inspeciona `bootstrap/handlers.go` nem `cmd/http/routes/declarable.go`
// procurando uses do RegistryKey: o fluxo natural de desmontagem é
// `service unregister` → apagar o pacote do verbo → `handler unregister` →
// `route unregister`, e o compile error guia ao próximo passo quando o pacote
// for apagado. Rodar apenas este comando sem seguir a sequência deixa o
// `Resolve[T](reg, <pkg>.RegistryKey)` válido em compile time, mas com `panic`
// em runtime — responsabilidade do dev.
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/services.go`).
func UnregisterService(opts UnregisterServiceOptions) (string, error) {
	plan, err := planService(RegisterServiceOptions(opts))
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

	registerFn, err := findFreeFunc(file, registerServicesFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}

	imp := findImportSpec(file, plan.importPath)
	if imp == nil {
		return "", fmt.Errorf("em %s: import %q ausente", plan.relFile, plan.importPath)
	}
	pkgIdent := importIdent(imp)
	if pkgIdent == "" {
		// blank/dot import — não é shape esperado pra um service register.
		return "", fmt.Errorf("em %s: import %q usa forma blank/dot não suportada", plan.relFile, plan.importPath)
	}

	provideStmt := findProvideStmt(registerFn, pkgIdent)
	if provideStmt == nil {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) ausente", plan.relFile, pkgIdent)
	}

	// A partir daqui validações passaram — segue mutação. `astutil.DeleteNamedImport`
	// aceita name vazio pra apagar imports bare, então cobre os dois shapes.
	aliasName := ""
	if imp.Name != nil {
		aliasName = imp.Name.Name
	}
	if !astutil.DeleteNamedImport(fset, file, aliasName, plan.importPath) {
		return "", fmt.Errorf("em %s: falha ao remover import %q", plan.relFile, plan.importPath)
	}
	registerFn.Body.List = removeStmt(registerFn.Body.List, provideStmt)
	// Fecha gap residual entre o último stmt e o Rbrace pra evitar linha em
	// branco no output (o printer respeita Pos originais; gofmt não recompacta
	// blank lines em function body).
	collapseTrailingBlank(fset, registerFn.Body)

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// collapseTrailingBlank reposiciona o Rbrace de um BlockStmt na linha
// imediatamente após o último Stmt restante (ou imediatamente após o Lbrace
// se o body ficou vazio), eliminando qualquer linha em branco residual que
// sobre quando o printer tenta respeitar as Pos originais dos nós removidos.
func collapseTrailingBlank(fset *token.FileSet, body *ast.BlockStmt) {
	if body == nil || !body.Rbrace.IsValid() {
		return
	}
	rbrace := fset.Position(body.Rbrace)
	if !rbrace.IsValid() {
		return
	}
	tf := fset.File(body.Rbrace)
	if tf == nil {
		return
	}

	var refLine int
	if len(body.List) == 0 {
		// Body vazio: ancorar no Lbrace pra deixar o `}` na linha seguinte.
		lbrace := fset.Position(body.Lbrace)
		if !lbrace.IsValid() {
			return
		}
		refLine = lbrace.Line
	} else {
		last := body.List[len(body.List)-1]
		lastEnd := fset.Position(last.End())
		if !lastEnd.IsValid() {
			return
		}
		refLine = lastEnd.Line
	}

	if rbrace.Line <= refLine+1 {
		return
	}
	newLine := refLine + 1
	if newLine < 1 || newLine > tf.LineCount() {
		return
	}
	body.Rbrace = tf.LineStart(newLine)
}

// findImportSpec localiza o *ast.ImportSpec cujo Path bate exatamente com
// importPath. Devolve nil se não houver. Diferente de `hasImportPath`, retorna
// o nó pra permitir inspeção do alias real e remoção posterior.
func findImportSpec(file *ast.File, importPath string) *ast.ImportSpec {
	quoted := strconv.Quote(importPath)
	for _, imp := range file.Imports {
		if imp.Path != nil && imp.Path.Value == quoted {
			return imp
		}
	}
	return nil
}

// findProvideStmt localiza o ExprStmt no corpo da função cuja chamada bate em
// `reg.Provide(<pkgIdent>.RegistryKey, _)`. Reusa `isProvideRegistryKeyCall`
// (definido em register_service.go), preservando o critério: chave única é o
// primeiro argumento; segundo argumento ignorado.
//
// Diferente de `hasProvideCall`, retorna o nó concreto pra permitir remoção
// via igualdade de ponteiro.
func findProvideStmt(fd *ast.FuncDecl, pkgIdent string) ast.Stmt {
	if fd == nil || fd.Body == nil {
		return nil
	}
	for _, stmt := range fd.Body.List {
		es, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := es.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		if isProvideRegistryKeyCall(call, pkgIdent) {
			return es
		}
	}
	return nil
}

// removeStmt devolve a slice sem o stmt alvo (igualdade de ponteiro). Preserva
// a ordem dos demais elementos.
func removeStmt(list []ast.Stmt, target ast.Stmt) []ast.Stmt {
	out := make([]ast.Stmt, 0, len(list))
	for _, s := range list {
		if s == target {
			continue
		}
		out = append(out, s)
	}
	return out
}
