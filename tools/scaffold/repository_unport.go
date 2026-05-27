// Package scaffold (área repository) — operação inversa de `repository port`.
//
// `repository unport` remove do arquivo do domínio tudo o que `repository port`
// adicionou: métodos canônicos (Schema/GetFilters/SoftDelete/SetFilters), campo
// não-exportado `filters`, interface `Repository`, e — em multi-tenant — campo
// `client` e método `SetClient`. Imports residuais sem uso são removidos. Auto-
// detecta multi-tenant pela presença simultânea do campo `client` e do método
// `SetClient`; estado parcial é erro.
package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// RepositoryUnportOptions parametriza RepositoryUnport.
type RepositoryUnportOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "OrgMembership").
	Domain string
}

// RepositoryUnport desfaz o que `RepositoryPort` enxertou no arquivo do
// domínio. Auto-detecta multi-tenant. Falha — sem mutar disco — se qualquer
// dos elementos esperados estiver ausente ou se houver estado parcial
// (client sem SetClient ou vice-versa).
//
// Retorna o caminho relativo do arquivo editado.
func RepositoryUnport(opts RepositoryUnportOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}

	relFile, absFile := domainPaths(opts.Root, opts.Domain)
	fset := token.NewFileSet()
	//nolint:gosec // G304: path derives from validated domain name, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(fset, absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", relFile, err)
	}

	st, err := findDomainStruct(file, opts.Domain)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	multiTenant, err := detectMultiTenant(file, st, opts.Domain)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	if err := assertNoMissing(file, st, opts.Domain, multiTenant); err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	// Mutação: métodos → campos → interface → imports.
	methods := []string{"Schema", "GetFilters", "SoftDelete", "SetFilters"}
	if multiTenant {
		methods = append(methods, "SetClient")
	}
	removeMethodsFromReceiver(file, opts.Domain, methods)

	removeUnexportedField(st, "filters")
	if multiTenant {
		removeUnexportedField(st, "client")
	}

	removeTypeDecl(file, "Repository")

	pruneUnusedImports(fset, file)

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// detectMultiTenant retorna true se o domínio está no shape multi-tenant
// (campo `client` + método `SetClient` ambos existem). Estado parcial — só
// um deles — é erro explícito. Ausência de ambos é o shape single-tenant.
func detectMultiTenant(file *ast.File, st *ast.StructType, domain string) (bool, error) {
	hasClientField := hasUnexportedField(st, "client")
	hasSetClient := hasMethod(file, domain, "SetClient")
	if hasClientField && hasSetClient {
		return true, nil
	}
	if hasClientField != hasSetClient {
		return false, fmt.Errorf("estado inconsistente: client/SetClient parcial (campo=%v, método=%v)", hasClientField, hasSetClient)
	}
	return false, nil
}

// assertNoMissing valida que todos os elementos a remover existem. Permite
// falhar antes de qualquer mutação, garantindo all-or-nothing.
func assertNoMissing(file *ast.File, st *ast.StructType, domain string, multiTenant bool) error {
	methods := []string{"Schema", "GetFilters", "SoftDelete", "SetFilters"}
	if multiTenant {
		methods = append(methods, "SetClient")
	}
	for _, m := range methods {
		if !hasMethod(file, domain, m) {
			return fmt.Errorf("método %s.%s ausente — domínio não está portado", domain, m)
		}
	}

	if !hasUnexportedField(st, "filters") {
		return fmt.Errorf("campo %s.filters ausente — domínio não está portado", domain)
	}
	if multiTenant && !hasUnexportedField(st, "client") {
		return fmt.Errorf("campo %s.client ausente — domínio não está portado", domain)
	}

	if !hasTypeDecl(file, "Repository") {
		return fmt.Errorf("tipo Repository ausente em %s — domínio não está portado", file.Name.Name)
	}
	return nil
}

// removeMethodsFromReceiver filtra os Decls de `file` removendo *ast.FuncDecl
// cujo receiver seja `recvType` (ponteiro ou valor) e nome esteja em `names`.
func removeMethodsFromReceiver(file *ast.File, recvType string, names []string) {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	kept := file.Decls[:0]
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && fd.Recv != nil && fd.Name != nil && nameSet[fd.Name.Name] && receiverMatches(fd.Recv, recvType) {
			continue
		}
		kept = append(kept, decl)
	}
	file.Decls = kept
}

// removeUnexportedField remove de `st` o campo cujo nome bate. No-op se
// inexistente (a verificação de presença é feita em assertNoMissing).
func removeUnexportedField(st *ast.StructType, name string) {
	idx := -1
	for i, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == name {
				idx = i
				break
			}
		}
		if idx >= 0 {
			break
		}
	}
	if idx < 0 {
		return
	}
	st.Fields.List = append(st.Fields.List[:idx], st.Fields.List[idx+1:]...)
}

// removeTypeDecl remove a declaração `type <typeName> ...` de `file`. Se o
// GenDecl agrupar múltiplos TypeSpecs, remove apenas o spec do tipo dado;
// caso contrário remove o GenDecl inteiro.
func removeTypeDecl(file *ast.File, typeName string) {
	kept := file.Decls[:0]
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			kept = append(kept, decl)
			continue
		}
		specs := gd.Specs[:0]
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if ok && ts.Name != nil && ts.Name.Name == typeName {
				continue
			}
			specs = append(specs, spec)
		}
		if len(specs) == 0 {
			// GenDecl inteiro removido (não preservar).
			continue
		}
		gd.Specs = specs
		kept = append(kept, gd)
	}
	file.Decls = kept
}

// repositoryHasCustomMethods retorna true se a interface `Repository` em file
// declara métodos custom além do embed `base_repository.BaseRepository[...]`.
// Usado pelo CLI para emitir warning ao usuário (decisão KD3).
func repositoryHasCustomMethods(file *ast.File) bool {
	it := findRepositoryInterface(file)
	if it == nil || it.Methods == nil {
		return false
	}
	for _, m := range it.Methods.List {
		// embed = anonymous field (sem nomes); método custom tem nome.
		if len(m.Names) > 0 {
			return true
		}
	}
	return false
}

// findRepositoryInterface localiza o *ast.InterfaceType da declaração
// `type Repository interface { ... }` em file. Retorna nil se ausente.
func findRepositoryInterface(file *ast.File) *ast.InterfaceType {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		if it := repositoryInterfaceFromSpecs(gd.Specs); it != nil {
			return it
		}
	}
	return nil
}

func repositoryInterfaceFromSpecs(specs []ast.Spec) *ast.InterfaceType {
	for _, spec := range specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Name == nil || ts.Name.Name != "Repository" {
			continue
		}
		if it, ok := ts.Type.(*ast.InterfaceType); ok {
			return it
		}
	}
	return nil
}
