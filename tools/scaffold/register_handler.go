package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	bootstrapHandlersFile = "handlers.go"
	registerHandlersFunc  = "registerHandlers"
)

// RegisterHandlerOptions parametriza RegisterHandler.
type RegisterHandlerOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "UsageRecord").
	// Determina o primeiro segmento do path do pacote a importar
	// (`cmd/http/handlers/<snake_domain>/<snake_service>`).
	Domain string
	// Service é o nome do use case em PascalCase (ex.: "Login", "Export").
	// Determina o segundo segmento do path e a chave do registry
	// (`<lowerCamelService>Handler`).
	Service string
}

// RegisterHandler patcha `bootstrap/handlers.go` adicionando o import do pacote do
// handler (com alias `<snake_domain sem underscores><snake_service sem underscores>handler`)
// e a chamada `reg.Provide(<alias>.RegistryKey, <alias>.New<Pascal>Handler(reg))`
// ao fim da função `registerHandlers`.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain e Service são PascalCase exportáveis.
//   - O arquivo do handler existe
//     (`cmd/http/handlers/<snake_domain>/<snake_service>/handler.go`) e contém
//     `const RegistryKey` + `func New<Pascal>Handler`.
//   - `bootstrap/handlers.go` existe e contém a função
//     `registerHandlers(reg *registry.Registry)`.
//   - O import e a linha de `Provide` ainda não existem (idempotente: re-rodar
//     sempre falha).
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/handlers.go`).
func RegisterHandler(opts RegisterHandlerOptions) (string, error) {
	plan, err := planHandler(opts)
	if err != nil {
		return "", err
	}
	if err := assertHandlerExists(plan.root, plan.snakeDomain, plan.snakeService, plan.pascalService); err != nil {
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

	if hasImportPath(file, plan.importPath) {
		return "", fmt.Errorf("em %s: import %q já presente", plan.relFile, plan.importPath)
	}
	registerFn, err := findFreeFunc(file, registerHandlersFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	if hasProvideCall(registerFn, plan.alias) {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) já presente", plan.relFile, plan.alias)
	}

	astutil.AddNamedImport(fset, file, plan.alias, plan.importPath)
	registerFn.Body.List = append(registerFn.Body.List, buildHandlerProvideStmt(plan.alias, plan.pascalService))

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// handlerPlan agrupa nomes derivados e caminhos pra evitar repetir a derivação
// nos vários passos de RegisterHandler.
type handlerPlan struct {
	root          string
	relFile       string
	absFile       string
	snakeDomain   string
	snakeService  string
	pascalService string
	importPath    string
	alias         string
}

func planHandler(opts RegisterHandlerOptions) (handlerPlan, error) {
	var plan handlerPlan
	if !IsValidExportedIdent(opts.Domain) {
		return plan, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Service) {
		return plan, fmt.Errorf("nome de service inválido (esperado PascalCase exportável): %q", opts.Service)
	}
	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	imp, err := newImportPaths(plan.root)
	if err != nil {
		return plan, err
	}
	plan.pascalService = opts.Service
	plan.snakeDomain = ToSnake(opts.Domain)
	plan.snakeService = ToSnake(opts.Service)
	plan.relFile = filepath.Join(bootstrapBasePath, bootstrapHandlersFile)
	plan.absFile = filepath.Join(plan.root, plan.relFile)
	plan.importPath = imp.join(handlersImportSubpath + "/" + plan.snakeDomain + "/" + plan.snakeService)
	// Alias uniforme: <snake_domain sem _><snake_service sem _>handler. Casa com
	// o padrão dos aliases legados (`authhandler`/`orghandler`, sufixo `handler`)
	// e impede colisão por construção — o par (snake_domain, snake_service) é
	// único por design da NAVE-70 (1:1 handler-por-service).
	plan.alias = strings.ReplaceAll(plan.snakeDomain, "_", "") +
		strings.ReplaceAll(plan.snakeService, "_", "") + "handler"
	return plan, nil
}

// assertHandlerExists confirma que o arquivo do handler existe e contém os
// símbolos que `handler register` precisa referenciar.
func assertHandlerExists(root, snakeDomain, snakeService, pascalService string) error {
	relFile := filepath.Join(handlersBasePath, snakeDomain, snakeService, "handler.go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("ler handler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	if !hasConstNamed(file, "RegistryKey") {
		return fmt.Errorf("em %s: const RegistryKey não encontrado", relFile)
	}
	constructor := "New" + pascalService + "Handler"
	if !hasFreeFuncNamed(file, constructor) {
		return fmt.Errorf("em %s: func %s não encontrado", relFile, constructor)
	}
	return nil
}

// buildHandlerProvideStmt constrói o ExprStmt:
//
//	reg.Provide(<alias>.RegistryKey, <alias>.New<Pascal>Handler(reg))
//
// Sempre passa apenas `reg` — exatamente o que `handler create` (NAVE-70) emite
// no constructor. Deps adicionais que o dev introduzir depois fazem a
// compilação quebrar até a chamada ser atualizada manualmente.
func buildHandlerProvideStmt(alias, pascalService string) *ast.ExprStmt {
	registryKey := &ast.SelectorExpr{
		X:   ast.NewIdent(alias),
		Sel: ast.NewIdent("RegistryKey"),
	}
	newHandler := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(alias),
			Sel: ast.NewIdent("New" + pascalService + "Handler"),
		},
		Args: []ast.Expr{ast.NewIdent("reg")},
	}
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("reg"),
				Sel: ast.NewIdent("Provide"),
			},
			Args: []ast.Expr{registryKey, newHandler},
		},
	}
}
