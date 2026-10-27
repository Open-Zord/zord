package scaffold

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
)

// ProjectionFieldRemoveOptions parametriza ProjectionFieldRemove.
type ProjectionFieldRemoveOptions struct {
	Root           string
	Domain         string
	ProjectionName string
	FieldName      string
}

// ProjectionFieldRemove apaga o campo da projection. Falha se o arquivo do
// domain, a projection ou o campo não existirem. Imports que ficarem sem uso
// são removidos.
func ProjectionFieldRemove(opts ProjectionFieldRemoveOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.ProjectionName) {
		return "", fmt.Errorf("nome de projection inválido: %q", opts.ProjectionName)
	}
	if !IsValidExportedIdent(opts.FieldName) {
		return "", fmt.Errorf("nome de campo inválido: %q", opts.FieldName)
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

	st, err := findDomainStruct(file, opts.ProjectionName)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	idx := indexOfField(st, opts.FieldName)
	if idx < 0 {
		return "", fmt.Errorf("campo %s.%s não existe", opts.ProjectionName, opts.FieldName)
	}
	st.Fields.List = append(st.Fields.List[:idx], st.Fields.List[idx+1:]...)

	pruneUnusedImports(fset, file)

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}
