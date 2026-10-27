package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	bootstrapPkgFile = "pkg.go"
	registerPkgFunc  = "registerPkg"
	pkgImportSubpath = "pkg"
)

// RegisterPkgOptions parametriza RegisterPkg/UnregisterPkg. É usado pelo
// gerenciador de pacotes (`zord package`) para conectar um pacote de `pkg/` no
// DI; a máquina de patch AST é a mesma de repository/service/handler.
type RegisterPkgOptions struct {
	// Root é a raiz do projeto. Vazio usa o diretório de trabalho atual.
	Root string
	// Package é o nome do pacote em `pkg/<Package>/` (também o identificador de
	// import, ex.: "cache"). Deve ser um identificador Go válido.
	Package string
	// Constructor é o construtor sem argumentos a registrar (ex.: "NewCache").
	// Só é lido por RegisterPkg; o unregister ignora o segundo argumento do
	// Provide (robusto a devs que evoluíram o construtor).
	Constructor string
}

// RegisterPkg patcha `bootstrap/http/pkg.go` adicionando o import de
// `<module>/pkg/<Package>` e a chamada `reg.Provide(<Package>.RegistryKey,
// <Package>.<Constructor>())` ao fim de `registerPkg`.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Package é um identificador Go válido.
//   - `pkg/<Package>/<Package>.go` existe e contém `const RegistryKey` + o
//     construtor informado.
//   - `bootstrap/http/pkg.go` existe e contém `registerPkg(...)`.
//   - O import e a linha de `Provide` ainda não existem (idempotente).
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/http/pkg.go`).
func RegisterPkg(opts RegisterPkgOptions) (string, error) {
	plan, err := planPkg(opts)
	if err != nil {
		return "", err
	}
	if err := assertPackageExists(plan.root, plan.pkgName, opts.Constructor); err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	//nolint:gosec // G304: path derives from the repo root, not raw user input
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
	registerFn, err := findFreeFunc(file, registerPkgFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	if hasProvideCall(registerFn, plan.pkgName) {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) já presente", plan.relFile, plan.pkgName)
	}

	astutil.AddImport(fset, file, plan.importPath)
	registerFn.Body.List = append(registerFn.Body.List, buildPkgProvideStmt(plan.pkgName, opts.Constructor))

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// UnregisterPkg desfaz o wiring de RegisterPkg: apaga o ImportSpec de
// `<module>/pkg/<Package>` e o ExprStmt `reg.Provide(<ident>.RegistryKey, _)`
// em `registerPkg`. Usa o alias REAL do import pra localizar o Provide (robusto
// a edição manual) e ignora o segundo argumento do Provide. Não inspeciona
// `pkg/<Package>/` no disco — funciona mesmo que o pacote já tenha sido apagado.
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/http/pkg.go`).
func UnregisterPkg(opts RegisterPkgOptions) (string, error) {
	plan, err := planPkg(opts)
	if err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	//nolint:gosec // G304: path derives from the repo root, not raw user input
	src, err := os.ReadFile(plan.absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", plan.relFile, err)
	}
	file, err := parser.ParseFile(fset, plan.absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", plan.relFile, err)
	}

	registerFn, err := findFreeFunc(file, registerPkgFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}

	imp := findImportSpec(file, plan.importPath)
	if imp == nil {
		return "", fmt.Errorf("em %s: import %q ausente", plan.relFile, plan.importPath)
	}
	pkgIdent := importIdent(imp)
	if pkgIdent == "" {
		return "", fmt.Errorf("em %s: import %q usa forma blank/dot não suportada", plan.relFile, plan.importPath)
	}

	provideStmt := findProvideStmt(registerFn, pkgIdent)
	if provideStmt == nil {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) ausente", plan.relFile, pkgIdent)
	}

	aliasName := ""
	if imp.Name != nil {
		aliasName = imp.Name.Name
	}
	if !astutil.DeleteNamedImport(fset, file, aliasName, plan.importPath) {
		return "", fmt.Errorf("em %s: falha ao remover import %q", plan.relFile, plan.importPath)
	}
	registerFn.Body.List = removeStmt(registerFn.Body.List, provideStmt)
	collapseTrailingBlank(fset, registerFn.Body)

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// pkgPlan agrupa os caminhos/nomes derivados de RegisterPkgOptions.
type pkgPlan struct {
	root       string
	relFile    string
	absFile    string
	pkgName    string
	importPath string
}

func planPkg(opts RegisterPkgOptions) (pkgPlan, error) {
	var plan pkgPlan
	if !token.IsIdentifier(opts.Package) {
		return plan, fmt.Errorf("nome de pacote inválido (esperado identificador Go): %q", opts.Package)
	}
	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	imp, err := newImportPaths(plan.root)
	if err != nil {
		return plan, err
	}
	plan.pkgName = opts.Package
	plan.relFile = filepath.Join(bootstrapBasePath, bootstrapHTTPSubpath, bootstrapPkgFile)
	plan.absFile = filepath.Join(plan.root, plan.relFile)
	plan.importPath = imp.join(pkgImportSubpath + "/" + plan.pkgName)
	return plan, nil
}

// assertPackageExists confirma que `pkg/<pkgName>/<pkgName>.go` existe e expõe
// os símbolos que `package add` referencia no wiring.
func assertPackageExists(root, pkgName, constructor string) error {
	relFile := filepath.Join(pkgImportSubpath, pkgName, pkgName+".go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("ler pacote %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	if !hasConstNamed(file, "RegistryKey") {
		return fmt.Errorf("em %s: const RegistryKey não encontrado", relFile)
	}
	if !hasFreeFuncNamed(file, constructor) {
		return fmt.Errorf("em %s: func %s não encontrado", relFile, constructor)
	}
	return nil
}

// buildPkgProvideStmt constrói o ExprStmt:
//
//	reg.Provide(<pkg>.RegistryKey, <pkg>.<constructor>())
//
// O construtor é chamado sem argumentos (baterias v1 têm construtor puro, no
// molde de pkg/idcreator). Deps futuras exigiriam evoluir este builder.
func buildPkgProvideStmt(pkgName, constructor string) *ast.ExprStmt {
	registryKey := &ast.SelectorExpr{
		X:   ast.NewIdent(pkgName),
		Sel: ast.NewIdent("RegistryKey"),
	}
	newPkg := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(pkgName),
			Sel: ast.NewIdent(constructor),
		},
	}
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("reg"),
				Sel: ast.NewIdent("Provide"),
			},
			Args: []ast.Expr{registryKey, newPkg},
		},
	}
}
