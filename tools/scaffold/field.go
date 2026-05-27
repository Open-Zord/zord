// Package scaffold (área field) implementa operações granulares sobre campos
// de structs Go de domínio (adicionar, remover) via AST.
package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

const domainBasePath = "internal/application/domain"

// FieldTag representa uma struct tag. Value pode ser vazio (ex.: db_pk).
type FieldTag struct {
	Key   string
	Value string
}

// FieldCanonicalTagOrder define a ordem em que as tags aparecem na struct.
var FieldCanonicalTagOrder = []string{
	"db", "json", "validate",
	"db_type", "db_size", "db_pk", "db_fk", "db_index",
}

// knownImports maps the short package name used in a qualified type
// (e.g. "time.Time") to its import path. Extended as scaffold evolves.
var knownImports = map[string]string{
	"time": "time",
}

// FieldAddOptions parametriza FieldAdd.
type FieldAddOptions struct {
	Root      string
	Domain    string     // PascalCase, ex.: "OrgMembership"
	FieldName string     // PascalCase, ex.: "UserID"
	TypeStr   string     // expressão de tipo Go, ex.: "string", "*time.Time"
	Tags      []FieldTag // já em FieldCanonicalTagOrder
}

// FieldAdd adiciona um novo campo à struct do domínio.
// Falha se o campo já existe, se o domínio/struct não for encontrado,
// ou se o tipo referenciar um pacote desconhecido.
func FieldAdd(opts FieldAddOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
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
	if hasField(st, opts.FieldName) {
		return "", fmt.Errorf("campo %s.%s já existe", opts.Domain, opts.FieldName)
	}

	st.Fields.List = append(st.Fields.List, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(opts.FieldName)},
		Type:  typeExpr,
		Tag:   buildTagLit(opts.Tags),
	})

	for _, imp := range imports {
		astutil.AddImport(fset, file, imp)
	}

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// FieldRemoveOptions parametriza FieldRemove.
type FieldRemoveOptions struct {
	Root      string
	Domain    string
	FieldName string
}

// FieldRemove apaga o campo da struct do domínio.
// Falha se o campo não existir. Imports que ficarem sem uso são removidos.
func FieldRemove(opts FieldRemoveOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
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
	st, err := findDomainStruct(file, opts.Domain)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	idx := indexOfField(st, opts.FieldName)
	if idx < 0 {
		return "", fmt.Errorf("campo %s.%s não existe", opts.Domain, opts.FieldName)
	}
	st.Fields.List = append(st.Fields.List[:idx], st.Fields.List[idx+1:]...)

	pruneUnusedImports(fset, file)

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

func indexOfField(st *ast.StructType, fieldName string) int {
	for i, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == fieldName {
				return i
			}
		}
	}
	return -1
}

// pruneUnusedImports remove do file os imports cuja identidade local (alias
// explícito ou último segmento do path) não aparece em nenhum SelectorExpr.
// Imports em branco (`_`) ou dot (`.`) são preservados.
func pruneUnusedImports(fset *token.FileSet, file *ast.File) {
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	var paths []string
	for _, imp := range file.Imports {
		if imp.Name != nil && (imp.Name.Name == "_" || imp.Name.Name == ".") {
			continue
		}
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		var local string
		if imp.Name != nil {
			local = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			local = parts[len(parts)-1]
		}
		if !used[local] {
			paths = append(paths, path)
		}
	}
	for _, p := range paths {
		astutil.DeleteImport(fset, file, p)
	}
}

func domainPaths(root, typeName string) (rel, abs string) {
	if root == "" {
		root = "."
	}
	snake := ToSnake(typeName)
	rel = filepath.Join(domainBasePath, snake, snake+".go")
	abs = filepath.Join(root, rel)
	return rel, abs
}

func findDomainStruct(file *ast.File, typeName string) (*ast.StructType, error) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("tipo %s não é struct", typeName)
			}
			return st, nil
		}
	}
	return nil, fmt.Errorf("struct %s não encontrada", typeName)
}

func hasField(st *ast.StructType, fieldName string) bool {
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == fieldName {
				return true
			}
		}
	}
	return false
}

func buildTagLit(tags []FieldTag) *ast.BasicLit {
	if len(tags) == 0 {
		return nil
	}
	parts := make([]string, 0, len(tags))
	for _, t := range tags {
		parts = append(parts, fmt.Sprintf(`%s:%s`, t.Key, strconv.Quote(t.Value)))
	}
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: "`" + strings.Join(parts, " ") + "`",
	}
}

func collectImports(e ast.Expr) ([]string, error) {
	var imps []string
	seen := map[string]bool{}
	var walkErr error
	ast.Inspect(e, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		path, ok := knownImports[id.Name]
		if !ok {
			walkErr = fmt.Errorf("pacote %q não suportado em tipos (suportados: %s)", id.Name, strings.Join(knownImportKeys(), ", "))
			return false
		}
		if !seen[path] {
			seen[path] = true
			imps = append(imps, path)
		}
		return true
	})
	return imps, walkErr
}

func knownImportKeys() []string {
	keys := make([]string, 0, len(knownImports))
	for k := range knownImports {
		keys = append(keys, k)
	}
	return keys
}

func writeFile(path string, fset *token.FileSet, file *ast.File) error {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return fmt.Errorf("formatar AST: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt: %w", err)
	}
	return os.WriteFile(path, formatted, 0o600)
}
