// Package scaffold (área response field) implementa operações granulares sobre
// campos da struct `Response` dentro de `response.go` de um use case.
//
// O `response.go` é gerado por `scaffold service create` e contém a struct
// `Response` (saída do use case). Os comandos aqui adicionam/removem campos
// dessa struct via AST puro, sempre com tag `json:"<snake>"`.
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

// ResponseFieldAddOptions parametriza ResponseFieldAdd.
type ResponseFieldAddOptions struct {
	Root      string
	Domain    string
	Verb      string
	FieldName string
	TypeStr   string
}

// ResponseFieldAdd adiciona um campo à struct `Response` do response.go.
// Sempre gera tag `json:"<snake_field>"`. Imports referenciados pelo tipo
// são adicionados automaticamente. Falha se o campo já existe.
func ResponseFieldAdd(opts ResponseFieldAddOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Verb) {
		return "", fmt.Errorf("nome de verbo inválido: %q", opts.Verb)
	}
	if !IsValidExportedIdent(opts.FieldName) {
		return "", fmt.Errorf("nome de campo inválido (esperado PascalCase exportável): %q", opts.FieldName)
	}
	if strings.TrimSpace(opts.TypeStr) == "" {
		return "", fmt.Errorf("tipo do campo não pode ser vazio")
	}

	typeExpr, err := parser.ParseExpr(opts.TypeStr)
	if err != nil {
		return "", fmt.Errorf("tipo inválido %q: %w", opts.TypeStr, err)
	}
	imports, err := collectImports(typeExpr)
	if err != nil {
		return "", err
	}

	relFile, absFile := responseFilePath(opts.Root, opts.Domain, opts.Verb)
	fset := token.NewFileSet()
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(fset, absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", relFile, err)
	}
	st, err := findResponseStruct(file)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}
	if hasField(st, opts.FieldName) {
		return "", fmt.Errorf("campo Response.%s já existe", opts.FieldName)
	}

	tags := []FieldTag{{Key: "json", Value: ToSnake(opts.FieldName)}}
	st.Fields.List = append(st.Fields.List, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(opts.FieldName)},
		Type:  typeExpr,
		Tag:   buildTagLit(tags),
	})

	for _, imp := range imports {
		astutil.AddImport(fset, file, imp)
	}

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// ResponseFieldRemoveOptions parametriza ResponseFieldRemove.
type ResponseFieldRemoveOptions struct {
	Root      string
	Domain    string
	Verb      string
	FieldName string
}

// ResponseFieldRemove apaga o campo da struct `Response`. Imports sem uso
// são removidos. Falha se o campo não existir.
func ResponseFieldRemove(opts ResponseFieldRemoveOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Verb) {
		return "", fmt.Errorf("nome de verbo inválido: %q", opts.Verb)
	}
	if !IsValidExportedIdent(opts.FieldName) {
		return "", fmt.Errorf("nome de campo inválido: %q", opts.FieldName)
	}

	relFile, absFile := responseFilePath(opts.Root, opts.Domain, opts.Verb)
	fset := token.NewFileSet()
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(fset, absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", relFile, err)
	}
	st, err := findResponseStruct(file)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}
	idx := indexOfField(st, opts.FieldName)
	if idx < 0 {
		return "", fmt.Errorf("campo Response.%s não existe", opts.FieldName)
	}
	st.Fields.List = append(st.Fields.List[:idx], st.Fields.List[idx+1:]...)

	pruneUnusedImports(fset, file)

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// responseFilePath devolve os caminhos relativo e absoluto do response.go
// do use case `<Domain>/<Verb>`.
func responseFilePath(root, domain, verb string) (rel, abs string) {
	if root == "" {
		root = "."
	}
	rel = filepath.Join(servicesBasePath, ToSnake(domain), ToSnake(verb), "response.go")
	abs = filepath.Join(root, rel)
	return rel, abs
}

func findResponseStruct(file *ast.File) (*ast.StructType, error) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Response" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("tipo Response não é struct")
			}
			return st, nil
		}
	}
	return nil, fmt.Errorf("struct Response não encontrada")
}
