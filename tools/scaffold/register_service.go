// Package scaffold (área register) edita os arquivos de wire-up em `bootstrap/` para registrar
// services, repositories e handlers no container DI (`pkg/registry`). Cada kind
// vive em um arquivo do pacote (service.go, repository.go, handler.go) e expõe
// uma função única que recebe Options e retorna o caminho relativo do arquivo
// editado.
//
// O scaffold deste pacote completa o ciclo iniciado por `service create`
// (NAVE-59), `repository create` (NAVE-58) e `handler create` (NAVE-70):
// criar o esqueleto compilável de cada camada é metade do trabalho; conectar
// no `bootstrap/` continua manual sem este pacote.
package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	bootstrapBasePath     = "bootstrap"
	bootstrapServicesFile = "services.go"
	registerServicesFunc  = "registerServices"
)

// RegisterServiceOptions parametriza RegisterService.
type RegisterServiceOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "OrgMembership").
	// Determina o segmento do path do pacote a importar
	// (`internal/application/services/<snake_domain>/...`).
	Domain string
	// Verb é o nome do verbo do use case em PascalCase (ex.: "Login",
	// "SelectOrg"). Determina o nome do subpacote e a chave do registry
	// (`<lowerCamelVerb>Service`).
	Verb string
}

// RegisterService patcha `bootstrap/services.go` adicionando o import do pacote do
// verbo e a chamada `reg.Provide(<pkg>.RegistryKey, <pkg>.NewService(log, idC))`
// ao fim da função `registerServices`.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain e Verb são PascalCase exportáveis.
//   - O arquivo do service existe
//     (`internal/application/services/<snake_domain>/<snake_verb>/service.go`)
//     e contém `const RegistryKey` + `func NewService`.
//   - `bootstrap/services.go` existe e contém a função
//     `registerServices(reg *registry.Registry)`.
//   - O import e a linha de `Provide` ainda não existem (idempotente: re-rodar
//     sempre falha).
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/services.go`).
func RegisterService(opts RegisterServiceOptions) (string, error) {
	plan, err := planService(opts)
	if err != nil {
		return "", err
	}
	if err := assertServiceExists(plan.root, plan.snakeDomain, plan.snakeVerb); err != nil {
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
	registerFn, err := findFreeFunc(file, registerServicesFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}

	// Detecta colisão de identificador entre o pacote do verbo e qualquer
	// import já presente no arquivo. Quando há colisão, aplica o alias
	// <snake_domain>_<snake_verb> tanto no import quanto na chamada de Provide.
	aliasToUse := ""
	if importIdentTaken(file, plan.snakeVerb) {
		plan.pkgIdent = plan.snakeDomain + "_" + plan.snakeVerb
		aliasToUse = plan.pkgIdent
	}

	if hasProvideCall(registerFn, plan.pkgIdent) {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) já presente", plan.relFile, plan.pkgIdent)
	}

	imp, err := newImportPaths(plan.root)
	if err != nil {
		return "", err
	}

	astutil.AddNamedImport(fset, file, aliasToUse, plan.importPath)
	ensureServiceLocals(fset, file, registerFn, imp)
	registerFn.Body.List = append(registerFn.Body.List, buildServiceProvideStmt(plan.pkgIdent))

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// ensureServiceLocals garante que `registerServices` resolva os primitivos
// `log` e `idC` no topo do corpo, exatamente como o código gerado por
// `service create` espera (`NewService(log, idC)`). Idempotente: só insere o
// que falta. Mantém o scaffold compatível com um `registerServices` de corpo
// vazio (caso do repo recém-gerado) sem exigir que o bootstrap traga locais
// não usados.
func ensureServiceLocals(fset *token.FileSet, file *ast.File, registerFn *ast.FuncDecl, imp importPaths) {
	var prefix []ast.Stmt
	if !hasLocalAssign(registerFn, "log") {
		astutil.AddImport(fset, file, imp.join(servicesImportSubpath))
		astutil.AddImport(fset, file, imp.join("pkg/logger"))
		prefix = append(prefix, buildResolveAssign("log", "services", "Logger", "logger"))
	}
	if !hasLocalAssign(registerFn, "idC") {
		astutil.AddImport(fset, file, imp.join(servicesImportSubpath))
		astutil.AddImport(fset, file, imp.join("pkg/idCreator"))
		prefix = append(prefix, buildResolveAssign("idC", "services", "IdCreator", "idCreator"))
	}
	if len(prefix) > 0 {
		registerFn.Body.List = append(prefix, registerFn.Body.List...)
	}
}

// hasLocalAssign verifica se o corpo da função já declara via `:=` uma variável
// de nome `name` (ex.: `log`/`idC`), evitando reinserção.
func hasLocalAssign(fn *ast.FuncDecl, name string) bool {
	if fn.Body == nil {
		return false
	}
	for _, stmt := range fn.Body.List {
		as, ok := stmt.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE {
			continue
		}
		for _, lhs := range as.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
				return true
			}
		}
	}
	return false
}

// buildResolveAssign monta `<name> := registry.Resolve[<pkg>.<typ>](reg, <keyPkg>.RegistryKey)`.
func buildResolveAssign(name, pkg, typ, keyPkg string) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(name)},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.IndexExpr{
					X: &ast.SelectorExpr{
						X:   ast.NewIdent("registry"),
						Sel: ast.NewIdent("Resolve"),
					},
					Index: &ast.SelectorExpr{
						X:   ast.NewIdent(pkg),
						Sel: ast.NewIdent(typ),
					},
				},
				Args: []ast.Expr{
					ast.NewIdent("reg"),
					&ast.SelectorExpr{
						X:   ast.NewIdent(keyPkg),
						Sel: ast.NewIdent("RegistryKey"),
					},
				},
			},
		},
	}
}

// servicePlan agrupa nomes derivados e caminhos pra evitar repetir a derivação
// nos vários passos de RegisterService.
type servicePlan struct {
	root        string
	relFile     string
	absFile     string
	snakeDomain string
	snakeVerb   string
	importPath  string
	pkgIdent    string
}

func planService(opts RegisterServiceOptions) (servicePlan, error) {
	var plan servicePlan
	if !IsValidExportedIdent(opts.Domain) {
		return plan, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Verb) {
		return plan, fmt.Errorf("nome de verbo inválido (esperado PascalCase exportável): %q", opts.Verb)
	}
	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	imp, err := newImportPaths(plan.root)
	if err != nil {
		return plan, err
	}
	plan.snakeDomain = ToSnake(opts.Domain)
	plan.snakeVerb = ToSnake(opts.Verb)
	plan.relFile = filepath.Join(bootstrapBasePath, bootstrapServicesFile)
	plan.absFile = filepath.Join(plan.root, plan.relFile)
	plan.importPath = imp.join(servicesImportSubpath + "/" + plan.snakeDomain + "/" + plan.snakeVerb)
	// CP3 substituirá pkgIdent por alias quando houver colisão; por ora o
	// identificador é sempre o nome bare do pacote.
	plan.pkgIdent = plan.snakeVerb
	return plan, nil
}

// findFreeFunc localiza uma função top-level (sem receiver) com o nome dado.
// Retorna erro se a função não existir ou não tiver corpo.
func findFreeFunc(file *ast.File, funcName string) (*ast.FuncDecl, error) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name == nil {
			continue
		}
		if fd.Name.Name != funcName {
			continue
		}
		if fd.Body == nil {
			return nil, fmt.Errorf("func %s não tem corpo", funcName)
		}
		return fd, nil
	}
	return nil, fmt.Errorf("func %s não encontrada", funcName)
}

// hasImportPath devolve true se o arquivo já importa o path informado, com ou
// sem alias.
func hasImportPath(file *ast.File, importPath string) bool {
	quoted := strconv.Quote(importPath)
	for _, imp := range file.Imports {
		if imp.Path != nil && imp.Path.Value == quoted {
			return true
		}
	}
	return false
}

// importIdentTaken devolve true se algum import existente já expõe o
// identificador `ident` (via alias explícito ou via basename do path).
//
// Convenção usada: assume que o nome do pacote bate com o último segmento do
// path do import quando não há alias. Cobre 100% dos imports gerados pelo
// scaffold; pacotes cujo nome real divirja do basename precisariam de
// inspeção do AST do alvo — não há caso real assim hoje.
func importIdentTaken(file *ast.File, ident string) bool {
	for _, imp := range file.Imports {
		if got := importIdent(imp); got == ident {
			return true
		}
	}
	return false
}

func importIdent(imp *ast.ImportSpec) string {
	if imp.Name != nil {
		switch imp.Name.Name {
		case "":
			// nunca acontece em ImportSpec parseado, mas defensivo
		case "_", ".":
			// blank/dot imports não expõem identificador
			return ""
		default:
			return imp.Name.Name
		}
	}
	if imp.Path == nil {
		return ""
	}
	path, err := strconv.Unquote(imp.Path.Value)
	if err != nil {
		return ""
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// hasProvideCall varre o corpo da função e devolve true se existir uma chamada
// `reg.Provide(<pkgIdent>.RegistryKey, ...)`. Não inspeciona o segundo argumento
// — a primeira chave é única e suficiente pra desempate.
func hasProvideCall(fd *ast.FuncDecl, pkgIdent string) bool {
	if fd == nil || fd.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok && isProvideRegistryKeyCall(call, pkgIdent) {
			found = true
			return false
		}
		return true
	})
	return found
}

// isProvideRegistryKeyCall devolve true para uma chamada da forma
// `reg.Provide(<pkgIdent>.RegistryKey, _)` — qualquer segundo argumento.
func isProvideRegistryKeyCall(call *ast.CallExpr, pkgIdent string) bool {
	fun, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || fun.Sel == nil || fun.Sel.Name != "Provide" {
		return false
	}
	recv, ok := fun.X.(*ast.Ident)
	if !ok || recv.Name != "reg" {
		return false
	}
	if len(call.Args) < 1 {
		return false
	}
	key, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok || key.Sel == nil || key.Sel.Name != "RegistryKey" {
		return false
	}
	id, ok := key.X.(*ast.Ident)
	return ok && id.Name == pkgIdent
}

// buildServiceProvideStmt constrói o ExprStmt:
//
//	reg.Provide(<pkgIdent>.RegistryKey, <pkgIdent>.NewService(log, idC))
//
// Sempre passa apenas `log` e `idC` — exatamente o que `service create` (NAVE-59)
// emite no constructor. Deps adicionais que o dev introduzir depois fazem a
// compilação quebrar até a chamada ser atualizada manualmente.
func buildServiceProvideStmt(pkgIdent string) *ast.ExprStmt {
	registryKey := &ast.SelectorExpr{
		X:   ast.NewIdent(pkgIdent),
		Sel: ast.NewIdent("RegistryKey"),
	}
	newService := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(pkgIdent),
			Sel: ast.NewIdent("NewService"),
		},
		Args: []ast.Expr{
			ast.NewIdent("log"),
			ast.NewIdent("idC"),
		},
	}
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("reg"),
				Sel: ast.NewIdent("Provide"),
			},
			Args: []ast.Expr{registryKey, newService},
		},
	}
}
