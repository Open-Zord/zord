package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
)

// SchemaFieldSpec representa um campo do domínio relevante para a tabela.
type SchemaFieldSpec struct {
	Name    string // nome Go do campo (PascalCase)
	GoType  string // tipo base, ex.: "string", "int64", "time.Time"
	Pointer bool   // true se o tipo é *T (coluna nullable)
	DBName  string // tag db
	DBType  string // tag db_type (sobrescreve mapeamento default)
	DBSize  string // tag db_size (args para tipos parametrizados, ex.: "255", "16,6")
	DBPK    bool   // tag db_pk:"true"
	DBFK    string // tag db_fk:"<tabela>.<coluna>"
	DBIndex string // tag db_index:"" | "true" | "unique"
}

// readDomain lê o arquivo de domínio e extrai os campos com tag `db` em SchemaFieldSpec.
// Campos sem tag `db` são ignorados (não são colunas).
func readDomain(root, domain string) ([]SchemaFieldSpec, error) {
	snake := ToSnake(domain)
	rel := filepath.Join(domainBasePath, snake, snake+".go")
	abs := filepath.Join(root, rel)

	fset := token.NewFileSet()
	//nolint:gosec // G304: path derives from validated domain name
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", rel, err)
	}
	file, err := parser.ParseFile(fset, abs, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}
	st, err := findStruct(file, domain)
	if err != nil {
		return nil, fmt.Errorf("em %s: %w", rel, err)
	}

	var specs []SchemaFieldSpec
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 || f.Tag == nil {
			continue
		}
		raw, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			return nil, fmt.Errorf("tag inválida em %s: %w", domain, err)
		}
		tag := reflect.StructTag(raw)
		dbName := tag.Get("db")
		if dbName == "" {
			continue
		}
		goType, pointer, ok := extractType(f.Type)
		if !ok {
			return nil, fmt.Errorf("campo %s.%s: tipo não suportado", domain, f.Names[0].Name)
		}
		// presença da tag (mesmo com valor vazio) indica PK. Alinha com a convenção
		// emitida por `scaffold field add --tag-db-pk` na fatia 1, que produz `db_pk:""`.
		_, hasPK := tag.Lookup("db_pk")
		for _, id := range f.Names {
			specs = append(specs, SchemaFieldSpec{
				Name:    id.Name,
				GoType:  goType,
				Pointer: pointer,
				DBName:  dbName,
				DBType:  tag.Get("db_type"),
				DBSize:  tag.Get("db_size"),
				DBPK:    hasPK,
				DBFK:    tag.Get("db_fk"),
				DBIndex: tag.Get("db_index"),
			})
		}
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("domínio %s não tem campos com tag db", domain)
	}
	return specs, nil
}

func findStruct(file *ast.File, typeName string) (*ast.StructType, error) {
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

// extractType retorna o tipo base (sem o `*`) e se o tipo original era pointer.
// Suporta identifiers ("string"), pointers ("*string") e seletores qualificados
// ("time.Time", "*time.Time"). Outros tipos (slices, maps, structs) retornam ok=false.
func extractType(e ast.Expr) (typeStr string, pointer, ok bool) {
	if star, isStar := e.(*ast.StarExpr); isStar {
		inner, _, innerOK := extractType(star.X)
		if !innerOK {
			return "", false, false
		}
		return inner, true, true
	}
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name, false, true
	case *ast.SelectorExpr:
		x, isIdent := t.X.(*ast.Ident)
		if !isIdent {
			return "", false, false
		}
		return x.Name + "." + t.Sel.Name, false, true
	}
	return "", false, false
}
