// Package scaffold (área handler delete) fecha o ciclo de desmontagem de um
// handler iniciado por `handler unregister` (NAVE-90). `handler delete`
// (NAVE-98) apaga a pasta `cmd/http/handlers/<snake_domain>/<snake_service>/`
// com guardas que recusam a operação enquanto o ecossistema do handler ainda
// tem dependências vivas: wire-up em `bootstrap/handlers.go` ou rota em
// `cmd/http/routes/<snake_domain>.go`.
package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

// HandlerDeleteOptions parametriza HandlerDelete.
type HandlerDeleteOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "OrgMembership").
	Domain string
	// Service é o nome do use case em PascalCase (ex.: "Login", "SelectOrg").
	Service string
}

// HandlerDelete apaga `cmd/http/handlers/<snake_domain>/<snake_service>/` via
// `os.RemoveAll` após validar, em ordem, que:
//
//  1. Domain e Service são PascalCase exportáveis.
//  2. A pasta do handler existe.
//  3. Não há wire-up residual em `bootstrap/handlers.go` (import OU Provide).
//     Bootstrap ausente conta como OK; bootstrap presente sem
//     `registerHandlers` também conta como OK (não há wire-up possível).
//  4. Não há rota residual em `cmd/http/routes/<snake_domain>.go`: campo
//     `<lowerCamel>Handler` na struct `<Pascal>Route`, import do pacote do
//     handler, ou uso `r.<lowerCamel>Handler.Handle` em Declare*Routes.
//     Route file ausente conta como OK.
//
// Falha sem mutar disco na primeira validação que falhar. A ordem é
// pasta → wire-up → rota: pasta primeiro porque é a invariante mais barata
// e dá a mensagem mais útil ("nada pra apagar"); wire-up antes da rota
// porque `Resolve` em runtime panica enquanto rota residual quebra build.
//
// Retorna o caminho relativo da pasta apagada.
func HandlerDelete(opts HandlerDeleteOptions) (string, error) {
	plan, err := planHandler(RegisterHandlerOptions(opts))
	if err != nil {
		return "", err
	}

	relDir := filepath.Join(handlersBasePath, plan.snakeDomain, plan.snakeService)
	absDir := filepath.Join(plan.root, relDir)

	if _, err := os.Stat(absDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("handler %s não existe", relDir)
		}
		return "", fmt.Errorf("stat %s: %w", absDir, err)
	}

	if err := assertNoHandlerWireUp(plan); err != nil {
		return "", err
	}

	if err := assertNoHandlerRoute(plan); err != nil {
		return "", err
	}

	if err := os.RemoveAll(absDir); err != nil {
		return "", fmt.Errorf("remover %s: %w", relDir, err)
	}
	return relDir, nil
}

// assertNoHandlerWireUp parseia `bootstrap/handlers.go` (se existir) e devolve
// erro se ainda houver import OU linha de Provide associada ao handler.
// Reusa `findImportSpec` e `findProvideStmt` definidos em
// unregister_service.go (compartilhados pelo par register/unregister de
// service e handler).
//
// Casos OK (retornam nil):
//   - `bootstrap/handlers.go` ausente — repo ainda sem bootstrap.
//   - Arquivo presente mas sem `registerHandlers` — não há wire-up possível.
//   - Arquivo presente, função presente, mas nem import nem Provide referem
//     o handler.
//
// Alias canônico (NAVE-70) é único por construção, sem detecção dual de
// formato: handlers legados com alias hand-editado ficam fora do alvo.
func assertNoHandlerWireUp(plan handlerPlan) error {
	src, err := os.ReadFile(plan.absFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ler %s: %w", plan.relFile, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, plan.absFile, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", plan.relFile, err)
	}

	if imp := findImportSpec(file, plan.importPath); imp != nil {
		return fmt.Errorf("em %s: import %q ainda presente — rode scaffold handler unregister %s %s antes", plan.relFile, plan.importPath, plan.snakeDomain, plan.snakeService)
	}

	registerFn, ferr := findFreeFunc(file, registerHandlersFunc)
	if ferr != nil {
		return nil //nolint:nilerr // ausência da função é intencionalmente OK
	}

	if findProvideStmt(registerFn, plan.alias) != nil {
		return fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) ainda presente — rode scaffold handler unregister %s %s antes", plan.relFile, plan.alias, plan.snakeDomain, plan.snakeService)
	}
	return nil
}

// assertNoHandlerRoute parseia `cmd/http/routes/<snake_domain>.go` (se
// existir) e devolve erro se a rota ainda referencia o handler de qualquer
// uma das três formas que `route add` (NAVE-74) introduz:
//
//   - Campo `<lowerCamel>Handler` na struct `<Pascal>Route`.
//   - Import do pacote do handler (`<module>/cmd/http/handlers/<snake_domain>/<snake_service>`).
//   - Chamada `r.<lowerCamel>Handler.Handle` em DeclarePrivateRoutes ou
//     DeclarePublicRoutes.
//
// Route file ausente conta como OK (handler nunca foi roteado). Struct
// ausente conta como OK (Route hand-editada com outro shape — fora do alvo,
// igual a `handler unregister` ignora handlers legados).
//
// A mensagem aponta edição manual do arquivo de rota; o comando recíproco
// `scaffold route remove` ainda não existe (NAVE-92, backlog).
func assertNoHandlerRoute(plan handlerPlan) error {
	relRoute := filepath.Join(routesBasePath, plan.snakeDomain+".go")
	absRoute := filepath.Join(plan.root, relRoute)

	//nolint:gosec // G304: path derives from validated identifier (snake_domain), not raw user input
	src, err := os.ReadFile(absRoute)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ler %s: %w", relRoute, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absRoute, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relRoute, err)
	}

	fieldName := ToLowerCamel(plan.pascalService) + "Handler"

	if structHasFieldNamed(file, fieldName) {
		return fmt.Errorf("em %s: campo %s ainda presente na struct da Route — remova o registro da rota antes (edição manual; scaffold route remove é backlog)", relRoute, fieldName)
	}

	if findImportSpec(file, plan.importPath) != nil {
		return fmt.Errorf("em %s: import %q ainda presente — remova o registro da rota antes (edição manual; scaffold route remove é backlog)", relRoute, plan.importPath)
	}

	if fileHasHandlerCall(file, fieldName) {
		return fmt.Errorf("em %s: r.%s.Handle ainda usado em Declare*Routes — remova o registro da rota antes (edição manual; scaffold route remove é backlog)", relRoute, fieldName)
	}

	return nil
}

// structHasFieldNamed varre todas as struct types no arquivo procurando um
// campo com o nome dado. Não filtra pelo nome do type para evitar acoplar
// `handler delete` ao shape canônico da Route (NAVE-74) — se o campo
// `<lowerCamel>Handler` existe em qualquer struct do arquivo de rota, o
// handler ainda está roteado.
func structHasFieldNamed(file *ast.File, fieldName string) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		st, ok := n.(*ast.StructType)
		if !ok {
			return true
		}
		if hasFieldNamed(st, fieldName) {
			found = true
			return false
		}
		return true
	})
	return found
}

// fileHasHandlerCall varre todas as funções do arquivo procurando uma
// chamada `r.<fieldName>.Handle`. Como `hasHandlerCall` opera sobre um
// FuncDecl específico, aqui iteramos sobre todos os FuncDecls do arquivo
// para cobrir DeclarePrivateRoutes, DeclarePublicRoutes e qualquer outro
// método que o dev tenha adicionado.
func fileHasHandlerCall(file *ast.File, fieldName string) bool {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if hasHandlerCall(fd, fieldName) {
			return true
		}
	}
	return false
}
