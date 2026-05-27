// Package scaffold (área request validator) liga/desliga validação de um
// use case regenerando o `request.go` por inteiro.
//
// Em vez de mutar o AST do arquivo existente — o que provoca cruzamentos
// entre doc comments e nodes recém-inseridos no go/printer — extraímos a
// lista de campos da struct `Data` do request atual e reescrevemos o
// arquivo do zero via `buildRequestFile`/`buildRequestFileWithValidator`.
// Comments e ordem dos campos do Data são preservados, mas posições
// internas viram limpas.
package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
)

// RequestValidatorOptions parametriza RequestValidatorSet e RequestValidatorUnset.
type RequestValidatorOptions struct {
	Root   string
	Domain string
	Verb   string
}

// RequestValidatorSet liga validação no request.go do use case regenerando
// o arquivo na variante com validator. Falha se o validator já estiver
// configurado.
func RequestValidatorSet(opts RequestValidatorOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Verb) {
		return "", fmt.Errorf("nome de verbo inválido: %q", opts.Verb)
	}

	relFile, absFile := requestFilePath(opts.Root, opts.Domain, opts.Verb)
	pkg, dataFields, hasValidator, err := readRequestState(absFile, relFile)
	if err != nil {
		return "", err
	}
	if hasValidator {
		return "", fmt.Errorf("validator já configurado em %s", relFile)
	}

	imp, err := newImportPaths(opts.Root)
	if err != nil {
		return "", err
	}
	src, err := buildRequestFileWithFields(pkg, opts.Verb, dataFields, true, imp)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(absFile, src, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", relFile, err)
	}
	return relFile, nil
}

// RequestValidatorUnset desliga validação no request.go regenerando o
// arquivo na variante sem validator. Falha se o validator não estiver
// configurado.
func RequestValidatorUnset(opts RequestValidatorOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido: %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Verb) {
		return "", fmt.Errorf("nome de verbo inválido: %q", opts.Verb)
	}

	relFile, absFile := requestFilePath(opts.Root, opts.Domain, opts.Verb)
	pkg, dataFields, hasValidator, err := readRequestState(absFile, relFile)
	if err != nil {
		return "", err
	}
	if !hasValidator {
		return "", fmt.Errorf("validator não configurado em %s", relFile)
	}

	src, err := buildRequestFileWithFields(pkg, opts.Verb, dataFields, false, importPaths{})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(absFile, src, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", relFile, err)
	}
	return relFile, nil
}

// readRequestState lê e parseia o request.go, extraindo:
//   - nome do pacote
//   - lista de campos da struct Data (preservados com tipo e tags)
//   - se o validator está configurado (campo `validator` no struct Request)
func readRequestState(absFile, relFile string) (string, []*ast.Field, bool, error) {
	//nolint:gosec // G304: path derives from validated identifiers
	src, err := os.ReadFile(absFile)
	if err != nil {
		return "", nil, false, fmt.Errorf("ler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.ParseComments)
	if err != nil {
		return "", nil, false, fmt.Errorf("parse %s: %w", relFile, err)
	}

	dataStruct, err := findRequestDataStruct(file)
	if err != nil {
		return "", nil, false, fmt.Errorf("em %s: %w", relFile, err)
	}
	var dataFields []*ast.Field
	for _, f := range dataStruct.Fields.List {
		// Limpa Doc/Comment pra evitar arrasto de posições antigas no rebuild.
		clone := &ast.Field{
			Names: append([]*ast.Ident(nil), f.Names...),
			Type:  f.Type,
		}
		if f.Tag != nil {
			clone.Tag = &ast.BasicLit{Kind: f.Tag.Kind, Value: f.Tag.Value}
		}
		for _, n := range clone.Names {
			n.NamePos = token.NoPos
		}
		dataFields = append(dataFields, clone)
	}

	reqStruct, err := findRequestStruct(file)
	if err != nil {
		return "", nil, false, fmt.Errorf("em %s: %w", relFile, err)
	}
	return file.Name.Name, dataFields, hasField(reqStruct, "validator"), nil
}

// findRequestStruct localiza a struct `Request` num request.go.
func findRequestStruct(file *ast.File) (*ast.StructType, error) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Request" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("tipo Request não é struct")
			}
			return st, nil
		}
	}
	return nil, fmt.Errorf("struct Request não encontrada")
}

// buildRequestFileWithFields gera o conteúdo de request.go com os campos de
// Data fornecidos e, opcionalmente, com a infraestrutura de validator.
//
// Quando withValidator=false reproduz o resultado de buildRequestFile no
// service create — exceto pela ordem dos campos de Data, que é preservada da
// origem.
//
// Quando withValidator=true a struct Request ganha o campo
// `validator services.Validator`, o NewRequest recebe o parâmetro adicional e
// o body de Validate passa a iterar sobre `r.validator.ValidateStruct(r.Data)`.
func buildRequestFileWithFields(pkg, verb string, dataFields []*ast.Field, withValidator bool, imp importPaths) ([]byte, error) {
	dataDecl := TypeDecl("Data", &ast.StructType{Fields: &ast.FieldList{List: dataFields}})
	dataDecl.Doc = singleComment(fmt.Sprintf("// Data agrega os campos de entrada validáveis do use case %s.", verb))

	requestFields := []*ast.Field{Field("Data", StarOf(Ident("Data")))}
	if withValidator {
		requestFields = append(requestFields, &ast.Field{
			Names: []*ast.Ident{Ident("validator")},
			Type:  Sel("services", "Validator"),
		})
	}
	requestDecl := TypeDecl("Request", &ast.StructType{Fields: &ast.FieldList{List: requestFields}})
	requestDecl.Doc = singleComment("// Request encapsula Data para o Execute do Service.")

	// NewRequest params e literal de retorno.
	newReqParams := []*ast.Field{Field("data", StarOf(Ident("Data")))}
	retElts := []ast.Expr{&ast.KeyValueExpr{Key: Ident("Data"), Value: Ident("data")}}
	if withValidator {
		newReqParams = append(newReqParams, &ast.Field{
			Names: []*ast.Ident{Ident("validator")},
			Type:  Sel("services", "Validator"),
		})
		retElts = append(retElts, &ast.KeyValueExpr{Key: Ident("validator"), Value: Ident("validator")})
	}
	newReqReturn := &ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
		Type: Ident("Request"),
		Elts: retElts,
	}}
	newRequestDecl := FuncDecl(
		nil,
		"NewRequest",
		&ast.FieldList{List: newReqParams},
		FieldList(AnonField(StarOf(Ident("Request")))),
		[]ast.Stmt{ReturnStmt(newReqReturn)},
	)
	newRequestDecl.Doc = singleComment("// NewRequest constrói o Request.")

	// Validate body conforme variante.
	var validateBody []ast.Stmt
	var validateDocText string
	if withValidator {
		validateBody = validatorValidateStmts()
		validateDocText = "// Validate roda as regras do go-playground/validator sobre Data."
	} else {
		validateBody = []ast.Stmt{ReturnStmt(Ident("nil"))}
		validateDocText = "// Validate valida Data. Sem validator configurado, retorna nil."
	}
	validateDecl := FuncDecl(
		PointerReceiver("r", "Request"),
		"Validate",
		FieldList(),
		FieldList(AnonField(Ident("error"))),
		validateBody,
	)
	validateDecl.Doc = singleComment(validateDocText)

	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-request-build")

	var imports *ast.GenDecl
	if withValidator {
		imports = ImportGroups(padder, []string{imp.join(servicesImportSubpath)})
	}

	packagePos := padder.Take()
	padder.Gap(1)

	if imports != nil {
		// importação já posicionada por ImportGroups; só fixa um gap após.
		padder.Gap(1)
	}

	stampDocAndDecl(padder, dataDecl)
	padder.Gap(1)
	stampDocAndDecl(padder, requestDecl)
	padder.Gap(1)

	stampDocPositions(padder, newRequestDecl.Doc)
	newRequestDecl.Type.Func = padder.Take()
	newRequestDecl.Body.Lbrace = padder.Take()
	newRequestDecl.Body.Rbrace = padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, validateDecl.Doc)
	validateDecl.Type.Func = padder.Take()
	validateDecl.Body.Lbrace = padder.Take()
	validateDecl.Body.Rbrace = padder.Take()

	var decls []ast.Decl
	if imports != nil {
		decls = append(decls, imports)
	}
	decls = append(decls, dataDecl, requestDecl, newRequestDecl, validateDecl)

	stamped := []ast.Decl{dataDecl, requestDecl, newRequestDecl, validateDecl}

	file := &ast.File{
		Package: packagePos,
		Name:    Ident(pkg),
		Decls:   decls,
	}
	file.Comments = collectDocs(nil, stamped)

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("formatar AST: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w\n%s", err, buf.String())
	}
	if !bytes.HasSuffix(formatted, []byte("\n")) {
		formatted = append(formatted, '\n')
	}
	return formatted, nil
}

// validatorValidateStmts monta o body de Validate quando o validator está
// configurado:
//
//	errs := r.validator.ValidateStruct(r.Data)
//	for _, err := range errs {
//	    if err != nil {
//	        return err
//	    }
//	}
//	return nil
func validatorValidateStmts() []ast.Stmt {
	errsAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("errs")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.SelectorExpr{
					X:   Ident("r"),
					Sel: Ident("validator"),
				},
				Sel: Ident("ValidateStruct"),
			},
			Args: []ast.Expr{&ast.SelectorExpr{X: Ident("r"), Sel: Ident("Data")}},
		}},
	}

	ifErr := &ast.IfStmt{
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{ReturnStmt(Ident("err"))}},
	}

	rangeStmt := &ast.RangeStmt{
		Key:   Ident("_"),
		Value: Ident("err"),
		Tok:   token.DEFINE,
		X:     Ident("errs"),
		Body:  &ast.BlockStmt{List: []ast.Stmt{ifErr}},
	}

	return []ast.Stmt{
		errsAssign,
		rangeStmt,
		ReturnStmt(Ident("nil")),
	}
}
