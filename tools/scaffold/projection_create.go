// Package scaffold (área projection) adiciona structs auxiliares (projections)
// ao arquivo do domínio. Projections são tipos de retorno de queries agregadas
// — convivem com a struct raiz no mesmo arquivo, com tags json (sempre) + db
// (decidido por campo no field add).
package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// ProjectionCreateOptions parametriza ProjectionCreate.
type ProjectionCreateOptions struct {
	Root           string
	Domain         string // PascalCase, ex.: "UsageRecord"
	ProjectionName string // PascalCase, ex.: "ResourceSummary"
}

// ProjectionCreate anexa `type <ProjectionName> struct {}` ao final do arquivo
// do domínio. Falha se o arquivo do domínio não existir, se a struct raiz
// (<Domain>) não estiver presente, se <ProjectionName> colidir com <Domain>
// ou se já houver um tipo top-level com o mesmo nome.
func ProjectionCreate(opts ProjectionCreateOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.ProjectionName) {
		return "", fmt.Errorf("nome de projection inválido (esperado PascalCase exportável): %q", opts.ProjectionName)
	}
	if opts.ProjectionName == opts.Domain {
		return "", fmt.Errorf("projection não pode ter o mesmo nome do domínio: %q", opts.ProjectionName)
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

	if _, err := findDomainStruct(file, opts.Domain); err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}
	if existingTopLevelType(file, opts.ProjectionName) {
		return "", fmt.Errorf("tipo %s já existe em %s", opts.ProjectionName, relFile)
	}

	decl := TypeDecl(opts.ProjectionName, &ast.StructType{Fields: FieldList()})
	padder := NewLinePadder(fset, "scaffold-projection-create")
	padder.StampDecls(decl)
	file.Decls = append(file.Decls, decl)

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// existingTopLevelType reporta se há um TypeSpec top-level com o nome dado
// (struct, interface, alias — qualquer tipo).
func existingTopLevelType(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name.Name == name {
				return true
			}
		}
	}
	return false
}
