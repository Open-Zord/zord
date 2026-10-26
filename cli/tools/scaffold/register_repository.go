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
	bootstrapRepositoriesFile = "repositories.go"
	repositoriesBasePath      = "internal/repositories"
	repositoriesImportSubpath = "internal/repositories"
	registerRepositoriesFunc  = "registerRepositories"
)

// RegisterRepositoryOptions parametriza RegisterRepository.
type RegisterRepositoryOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Organization",
	// "OrgMembership"). Determina o segmento do path do pacote a importar
	// (`internal/repositories/<snake_domain>`).
	Domain string
}

// RegisterRepository patcha `bootstrap/http/repositories.go` adicionando o import do pacote
// do repositório (com alias `<snake_domain sem underscores>repo`) e a chamada
// `reg.Provide(<alias>.RegistryKey, <alias>.New<Pascal>Repository(db))` ao fim
// da função `registerRepositories`.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain é PascalCase exportável.
//   - O arquivo do repositório existe
//     (`internal/repositories/<snake_domain>/<snake_domain>.go`) e contém
//     `const RegistryKey` + `func New<Pascal>Repository`.
//   - `bootstrap/http/repositories.go` existe e contém a função
//     `registerRepositories(reg *registry.Registry)`.
//   - O import e a linha de `Provide` ainda não existem (idempotente: re-rodar
//     sempre falha).
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/http/repositories.go`).
func RegisterRepository(opts RegisterRepositoryOptions) (string, error) {
	plan, err := planRepository(opts)
	if err != nil {
		return "", err
	}
	if err := assertRepositoryExists(plan.root, plan.snakeDomain, plan.pascalDomain); err != nil {
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
	registerFn, err := findFreeFunc(file, registerRepositoriesFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	if hasProvideCall(registerFn, plan.alias) {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) já presente", plan.relFile, plan.alias)
	}

	astutil.AddNamedImport(fset, file, plan.alias, plan.importPath)
	registerFn.Body.List = append(registerFn.Body.List, buildRepositoryProvideStmt(plan.alias, plan.pascalDomain))

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// repositoryPlan agrupa nomes derivados e caminhos pra evitar repetir a
// derivação nos vários passos de RegisterRepository.
type repositoryPlan struct {
	root         string
	relFile      string
	absFile      string
	snakeDomain  string
	pascalDomain string
	importPath   string
	alias        string
}

func planRepository(opts RegisterRepositoryOptions) (repositoryPlan, error) {
	var plan repositoryPlan
	if !IsValidExportedIdent(opts.Domain) {
		return plan, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	imp, err := newImportPaths(plan.root)
	if err != nil {
		return plan, err
	}
	plan.pascalDomain = opts.Domain
	plan.snakeDomain = ToSnake(opts.Domain)
	plan.relFile = filepath.Join(bootstrapBasePath, bootstrapHTTPSubpath, bootstrapRepositoriesFile)
	plan.absFile = filepath.Join(plan.root, plan.relFile)
	plan.importPath = imp.join(repositoriesImportSubpath + "/" + plan.snakeDomain)
	// Alias uniforme: <snake_domain sem underscores> + "repo". Casa com o
	// padrão `orgmembershiprepo`/`refreshtokenrepo` dos imports atuais e
	// evita colisão por construção (pacotes em `internal/repositories/` jamais
	// se chamam `<snake>repo`).
	plan.alias = strings.ReplaceAll(plan.snakeDomain, "_", "") + "repo"
	return plan, nil
}

// assertRepositoryExists confirma que o arquivo do repository existe e contém
// os símbolos que `repository register` precisa referenciar.
func assertRepositoryExists(root, snakeDomain, pascalDomain string) error {
	relFile := filepath.Join(repositoriesBasePath, snakeDomain, snakeDomain+".go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("ler repository %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	if !hasConstNamed(file, "RegistryKey") {
		return fmt.Errorf("em %s: const RegistryKey não encontrado", relFile)
	}
	constructor := "New" + pascalDomain + "Repository"
	if !hasFreeFuncNamed(file, constructor) {
		return fmt.Errorf("em %s: func %s não encontrado", relFile, constructor)
	}
	return nil
}

// buildRepositoryProvideStmt constrói o ExprStmt:
//
//	reg.Provide(<alias>.RegistryKey, <alias>.New<Pascal>Repository(db))
//
// Sempre passa apenas `db` — exatamente o que `repository create` (NAVE-58)
// emite no constructor. Deps adicionais que o dev introduzir depois fazem a
// compilação quebrar até a chamada ser atualizada manualmente.
func buildRepositoryProvideStmt(alias, pascalDomain string) *ast.ExprStmt {
	registryKey := &ast.SelectorExpr{
		X:   ast.NewIdent(alias),
		Sel: ast.NewIdent("RegistryKey"),
	}
	newRepo := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(alias),
			Sel: ast.NewIdent("New" + pascalDomain + "Repository"),
		},
		Args: []ast.Expr{ast.NewIdent("db")},
	}
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("reg"),
				Sel: ast.NewIdent("Provide"),
			},
			Args: []ast.Expr{registryKey, newRepo},
		},
	}
}
