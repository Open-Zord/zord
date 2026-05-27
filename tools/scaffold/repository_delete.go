package scaffold

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

// RepositoryDeleteOptions parametriza RepositoryDelete.
type RepositoryDeleteOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Organization",
	// "OrgMembership"). Determina a pasta a remover
	// (`internal/repositories/<snake_domain>`).
	Domain string
}

// RepositoryDelete remove a pasta `internal/repositories/<snake_domain>/`
// (inteira, recursivamente), simétrica a `repository create` (NAVE-58).
//
// Validações (todas obrigatórias, falham sem mutar disco):
//
//   - Domain é PascalCase exportável.
//   - A pasta existe.
//   - Não há wire-up residual em `bootstrap/repositories.go`. Quando o
//     arquivo de bootstrap existe e contém a função `registerRepositories`,
//     procura via AST o ImportSpec do pacote (`<module>/internal/
//     repositories/<snake_domain>`) e a chamada
//     `reg.Provide(<alias>.RegistryKey, _)`. Se os dois existem, falha com
//     mensagem apontando `scaffold repository unregister <Domain>`. Usa o
//     alias REAL do ImportSpec pra robustez contra aliases encurtados à mão
//     (mesma estratégia do unregister, NAVE-89).
//
// Bootstrap ausente, função `registerRepositories` ausente, import ausente,
// ou Provide ausente — todos significam "sem wire-up residual", e o delete
// segue. Não há pra onde apontar; estado já é consistente.
//
// Não inspeciona services downstream procurando uses do RegistryKey: o
// fluxo natural é `unregister → delete → ajustar services` e o compile
// error guia o próximo passo. Mesma postura de `service unregister`
// (NAVE-88) e `repository unregister` (NAVE-89).
//
// Retorna o caminho relativo da pasta removida
// (`internal/repositories/<snake_domain>`).
func RepositoryDelete(opts RepositoryDeleteOptions) (string, error) {
	plan, err := planRepository(RegisterRepositoryOptions(opts))
	if err != nil {
		return "", err
	}

	relDir := filepath.Join(repositoriesBasePath, plan.snakeDomain)
	absDir := filepath.Join(plan.root, relDir)

	info, err := os.Stat(absDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("repository não encontrado: %s", relDir)
		}
		return "", fmt.Errorf("stat %s: %w", relDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository não é uma pasta: %s", relDir)
	}

	if err := assertNoRepositoryWireUp(plan); err != nil {
		return "", err
	}

	if err := os.RemoveAll(absDir); err != nil {
		return "", fmt.Errorf("remover %s: %w", relDir, err)
	}
	return relDir, nil
}

// assertNoRepositoryWireUp falha se `bootstrap/repositories.go` ainda
// referencia o repository pelo importPath E pela chamada reg.Provide.
// Tolerante a estados parciais: bootstrap ausente, função ausente, só
// import sem Provide, ou só Provide sem import — todos considerados "sem
// wire-up", porque não há nada pra `repository unregister` desfazer.
func assertNoRepositoryWireUp(plan repositoryPlan) error {
	src, err := os.ReadFile(plan.absFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("ler %s: %w", plan.relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), plan.absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", plan.relFile, err)
	}

	imp := findImportSpec(file, plan.importPath)
	if imp == nil {
		return nil
	}
	pkgIdent := importIdent(imp)
	if pkgIdent == "" {
		return nil
	}
	registerFn, err := findFreeFunc(file, registerRepositoriesFunc)
	if err != nil {
		// função ausente = sem wire-up
		return nil //nolint:nilerr // deliberado: estado consistente, delete segue
	}
	if findProvideStmt(registerFn, pkgIdent) == nil {
		return nil
	}
	return fmt.Errorf(
		"wire-up residual em %s: reg.Provide(%s.RegistryKey, ...) ainda presente — rode \"scaffold repository unregister %s\" primeiro",
		plan.relFile, pkgIdent, plan.pascalDomain,
	)
}
