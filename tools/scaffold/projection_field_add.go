package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"golang.org/x/tools/go/ast/astutil"
)

// ProjectionFieldAddOptions parametriza ProjectionFieldAdd.
type ProjectionFieldAddOptions struct {
	Root           string
	Domain         string // PascalCase, ex.: "UsageRecord"
	ProjectionName string // PascalCase, ex.: "ResourceSummary"
	FieldName      string // PascalCase, ex.: "ResourceType"
	TypeStr        string // expressão Go do tipo: "string", "float64", "*time.Time", "[]ResourceSummary"

	// DBTagSet sinaliza que o caller passou --tag-db (mesmo vazio é override).
	// Quando false, a tag db usa o snake do FieldName por default.
	DBTagSet   bool
	DBTagValue string

	// NoDBTag suprime a tag db (pra campos de struct composta que não vêm de
	// StructScan). Mutuamente exclusivo com DBTagSet — validado pelo caller.
	NoDBTag bool
}

// ProjectionFieldAdd adiciona um campo à struct projection. Tag json sempre
// presente (valor = snake do FieldName); tag db presente por default (snake
// do FieldName), override via DBTagValue, suprimida com NoDBTag.
//
// Tipos compostos suportados:
//   - escalares Go (string, int, int64, float64, bool, time.Time);
//   - ponteiros (*string, *time.Time);
//   - slice de outra projection do mesmo arquivo ([]ResourceSummary);
//   - ponteiro de outra projection do mesmo arquivo (*ResourceSummary).
//
// Slice e ponteiro de outra projection forçam a validação cruzada: a
// projection referenciada precisa existir como tipo top-level no mesmo
// arquivo do domain (criar primeiro a "raw", depois a "composta"). Slice
// e struct nunca recebem tag db automaticamente.
func ProjectionFieldAdd(opts ProjectionFieldAddOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.ProjectionName) {
		return "", fmt.Errorf("nome de projection inválido: %q", opts.ProjectionName)
	}
	if !IsValidExportedIdent(opts.FieldName) {
		return "", fmt.Errorf("nome de campo inválido (esperado PascalCase exportável): %q", opts.FieldName)
	}
	if opts.DBTagSet && opts.NoDBTag {
		return "", fmt.Errorf("DBTagSet e NoDBTag são incompatíveis")
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

	st, err := findDomainStruct(file, opts.ProjectionName)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}
	if hasField(st, opts.FieldName) {
		return "", fmt.Errorf("campo %s.%s já existe", opts.ProjectionName, opts.FieldName)
	}
	if err := assertProjectionRefs(file, typeExpr, opts.ProjectionName); err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	st.Fields.List = append(st.Fields.List, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(opts.FieldName)},
		Type:  typeExpr,
		Tag:   buildProjectionTagLit(opts.FieldName, typeExpr, opts.DBTagSet, opts.DBTagValue, opts.NoDBTag),
	})

	for _, imp := range imports {
		astutil.AddImport(fset, file, imp)
	}

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// buildProjectionTagLit monta a struct tag pro field da projection. Sempre
// inclui json:"<snake_field>". Inclui db a menos que NoDBTag esteja set ou
// o tipo seja composto (slice/struct/array).
func buildProjectionTagLit(fieldName string, typeExpr ast.Expr, dbTagSet bool, dbTagValue string, noDBTag bool) *ast.BasicLit {
	tags := []FieldTag{{Key: "json", Value: ToSnake(fieldName)}}
	if !noDBTag && isScannableType(typeExpr) {
		val := ToSnake(fieldName)
		if dbTagSet {
			val = dbTagValue
		}
		// db vem antes de json na ordem canônica.
		tags = append([]FieldTag{{Key: "db", Value: val}}, tags...)
	}
	return buildTagLit(tags)
}

// isScannableType retorna true quando o tipo pode vir direto de sqlx.StructScan
// (escalares + ponteiros pra escalares). Slices, arrays e structs nomeadas
// jamais ganham tag db automática — sqlx não popula esses tipos sem custom scanner.
func isScannableType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return true
	case *ast.StarExpr:
		return isScannableType(t.X)
	default:
		return false
	}
}

// assertProjectionRefs valida que referências a outras projections (Ident sem
// pacote, em []X ou *X) apontam pra um tipo top-level existente no mesmo
// arquivo. selfName (a projection sendo alterada) é ignorada — auto-referência
// via ponteiro/slice é decisão do dev e não cria ciclo de tipo no Go.
func assertProjectionRefs(file *ast.File, typeExpr ast.Expr, selfName string) error {
	var refs []string
	collectLocalTypeRefs(typeExpr, &refs)
	for _, ref := range refs {
		if ref == selfName {
			continue
		}
		if isBuiltinType(ref) {
			continue
		}
		if !existingTopLevelType(file, ref) {
			return fmt.Errorf("tipo %q referenciado por composição não existe no arquivo — crie-o antes via 'projection create'", ref)
		}
	}
	return nil
}

// collectLocalTypeRefs popula refs com nomes de Ident encontrados sob slices
// e ponteiros (composição local). Não inspeciona SelectorExpr (pacotes
// externos têm validação separada via collectImports).
func collectLocalTypeRefs(e ast.Expr, refs *[]string) {
	switch t := e.(type) {
	case *ast.ArrayType:
		collectLocalTypeRefs(t.Elt, refs)
	case *ast.StarExpr:
		collectLocalTypeRefs(t.X, refs)
	case *ast.Ident:
		*refs = append(*refs, t.Name)
	}
}

// isBuiltinType cobre os tipos Go embutidos que o scaffold aceita como
// escalares — usados em campos de projection sem precisar de declaração local.
//
//nolint:goconst // "string" é um nome de tipo Go embutido, não constante de domínio
func isBuiltinType(name string) bool {
	switch name {
	case "bool", "byte", "rune", "string",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"complex64", "complex128",
		"error", "any":
		return true
	}
	return false
}
