package scaffold

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"golang.org/x/tools/go/ast/astutil"
)

// UnregisterRepositoryOptions parametriza UnregisterRepository.
type UnregisterRepositoryOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Organization",
	// "OrgMembership"). Mesma convenção de `repository register` (NAVE-72).
	Domain string
}

// UnregisterRepository remove a ligação no DI feita por `repository register`:
// apaga o ImportSpec do pacote do repositório (`internal/repositories/
// <snake_domain>`, com alias `<snake_domain sem underscores>repo`) e o
// ExprStmt da chamada `reg.Provide(<alias>.RegistryKey, _)` em
// `registerRepositories`.
//
// O ImportSpec é localizado pelo path (`<module>/internal/repositories/
// <snake_domain>`); o alias REAL declarado no import é usado pra localizar o
// `reg.Provide`. Isso é robusto contra repos legados onde o dev encurtou o
// alias à mão (ex.: `orgrepo`/`userrepo` em vez do `organizationrepo`/
// `platformuserrepo` que NAVE-72 gera). Mesma estratégia de `service
// unregister` (NAVE-88). O segundo argumento de `reg.Provide` é ignorado:
// devs evoluem `NewXxxRepository(db)` adicionando deps ao constructor, e o
// unregister precisa funcionar após essa evolução.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain é PascalCase exportável.
//   - `bootstrap/repositories.go` existe e contém `registerRepositories(reg *registry.Registry)`.
//   - O import existe.
//   - A linha `reg.Provide(<alias>.RegistryKey, _)` existe em registerRepositories.
//
// Não inspeciona o pacote do repository no disco — o unregister deve funcionar
// mesmo que o dev já tenha apagado `internal/repositories/<snake>/`.
//
// Não inspeciona `bootstrap/services.go` nem outros arquivos procurando uses
// downstream do RegistryKey: o fluxo natural é unregister → apagar o pacote →
// ajustar services que dependiam dele, e o compile error guia ao próximo
// passo. Rodar apenas este comando sem seguir a sequência deixa
// `Resolve[T](reg, <alias>.RegistryKey)` válido em compile time mas com
// `panic` em runtime — responsabilidade do dev.
//
// Retorna o caminho relativo do arquivo editado (`bootstrap/repositories.go`).
func UnregisterRepository(opts UnregisterRepositoryOptions) (string, error) {
	plan, err := planRepository(RegisterRepositoryOptions(opts))
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

	registerFn, err := findFreeFunc(file, registerRepositoriesFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}

	imp := findImportSpec(file, plan.importPath)
	if imp == nil {
		return "", fmt.Errorf("em %s: import %q ausente", plan.relFile, plan.importPath)
	}
	// Usa o alias REAL do ImportSpec pra localizar o Provide — robusto contra
	// repos legados onde o dev encurtou o alias à mão (ex.: `orgrepo` em vez
	// do default `organizationrepo` que NAVE-72 gera). Mesma estratégia de
	// `service unregister` (NAVE-88).
	pkgIdent := importIdent(imp)
	if pkgIdent == "" {
		// blank/dot import — não é shape esperado pra um repository register.
		return "", fmt.Errorf("em %s: import %q usa forma blank/dot não suportada", plan.relFile, plan.importPath)
	}

	provideStmt := findProvideStmt(registerFn, pkgIdent)
	if provideStmt == nil {
		return "", fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) ausente", plan.relFile, pkgIdent)
	}

	// A partir daqui validações passaram — segue mutação. O alias do ImportSpec
	// real pode divergir do esperado (`plan.alias`) se algum dev tiver editado
	// à mão; passar o alias real pra DeleteNamedImport garante remoção correta
	// nesse caso, embora o Provide ainda precise bater com plan.alias acima.
	aliasName := ""
	if imp.Name != nil {
		aliasName = imp.Name.Name
	}
	if !astutil.DeleteNamedImport(fset, file, aliasName, plan.importPath) {
		return "", fmt.Errorf("em %s: falha ao remover import %q", plan.relFile, plan.importPath)
	}
	registerFn.Body.List = removeStmt(registerFn.Body.List, provideStmt)
	// Fecha gap residual entre o último stmt e o Rbrace (mesmo fix descoberto
	// em NAVE-88: printer respeita Pos originais; gofmt não recompacta blank
	// lines em function body).
	collapseTrailingBlank(fset, registerFn.Body)

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}
