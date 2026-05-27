// Package scaffold (área request field) implementa operações granulares sobre
// campos da struct `Data` dentro de `request.go` de um use case.
//
// O `request.go` é gerado por `scaffold service create` e contém a struct
// `Data` (entrada validável do use case). Os comandos aqui adicionam/removem
// campos dessa struct via AST puro, sempre com tag `json:"<snake>"` e
// opcionalmente `validate:"<rules>"`.
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

// RequestFieldAddOptions parametriza RequestFieldAdd.
type RequestFieldAddOptions struct {
	Root      string
	Domain    string // PascalCase, ex.: "OrgMembership"
	Verb      string // PascalCase, ex.: "Create"
	FieldName string // PascalCase, ex.: "Email"
	TypeStr   string // expressão de tipo Go, ex.: "string", "*time.Time"
	// Validate é o valor da tag validate (opcional, ex.: "required,email").
	// Vazio omite a tag.
	Validate string
}

// RequestFieldAdd adiciona um campo à struct `Data` do request do use case
// `<Domain>/<Verb>`. Tags geradas:
//
//   - sempre `json:"<snake_field>"`
//   - se Validate != "": `validate:"<value>"`
//
// Imports referenciados pelo tipo (ex.: `time.Time`) são adicionados
// automaticamente. Falha se o campo já existe ou se o request.go não tem a
// struct `Data`.
func RequestFieldAdd(opts RequestFieldAddOptions) (string, error) {
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

	relFile, absFile := requestFilePath(opts.Root, opts.Domain, opts.Verb)
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
	st, err := findRequestDataStruct(file)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}
	if hasField(st, opts.FieldName) {
		return "", fmt.Errorf("campo Data.%s já existe", opts.FieldName)
	}

	tags := []FieldTag{{Key: "json", Value: ToSnake(opts.FieldName)}}
	if v := strings.TrimSpace(opts.Validate); v != "" {
		tags = append(tags, FieldTag{Key: "validate", Value: v})
	}
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

// RequestFieldRemoveOptions parametriza RequestFieldRemove.
type RequestFieldRemoveOptions struct {
	Root      string
	Domain    string
	Verb      string
	FieldName string
}

// RequestFieldRemove apaga o campo da struct `Data` do request. Imports que
// ficarem sem uso são removidos. Falha se o campo não existir.
func RequestFieldRemove(opts RequestFieldRemoveOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Verb) {
		return "", fmt.Errorf("nome de verbo inválido: %q", opts.Verb)
	}
	if !IsValidExportedIdent(opts.FieldName) {
		return "", fmt.Errorf("nome de campo inválido: %q", opts.FieldName)
	}

	relFile, absFile := requestFilePath(opts.Root, opts.Domain, opts.Verb)
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
	st, err := findRequestDataStruct(file)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}
	idx := indexOfField(st, opts.FieldName)
	if idx < 0 {
		return "", fmt.Errorf("campo Data.%s não existe", opts.FieldName)
	}
	st.Fields.List = append(st.Fields.List[:idx], st.Fields.List[idx+1:]...)

	pruneUnusedImports(fset, file)

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// requestFilePath devolve os caminhos relativo e absoluto do request.go do
// use case `<Domain>/<Verb>`.
func requestFilePath(root, domain, verb string) (rel, abs string) {
	if root == "" {
		root = "."
	}
	rel = filepath.Join(servicesBasePath, ToSnake(domain), ToSnake(verb), "request.go")
	abs = filepath.Join(root, rel)
	return rel, abs
}

// findRequestDataStruct localiza a struct `Data` num request.go.
func findRequestDataStruct(file *ast.File) (*ast.StructType, error) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Data" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("tipo Data não é struct")
			}
			return st, nil
		}
	}
	return nil, fmt.Errorf("struct Data não encontrada")
}
