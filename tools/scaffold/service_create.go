// Package scaffold (área service) constrói o esqueleto de um use case via AST
// puro no padrão zord-microframework: três arquivos por verbo —
// `request.go`, `service.go` e `response.go` — na pasta
// `internal/application/services/<snake_domain>/<snake_verb>/`.
//
// A divisão segue o template zord: o `Request` agrega `Data` + validador
// opcional; o `Service` consome um `*Request` via `Execute(ctx, *Request) error`
// e expõe a saída por `GetResponse() (*Response, error)`; falhas viram
// `services.AppError`; o `Response` é a struct de saída do use case.
//
// O foco é estrutural: nenhum port do domínio é injetado automaticamente — o
// dev adiciona dependências à mão depois (campos no Data via
// `scaffold request field add`, campos no Response via
// `scaffold response field add`, validador via `scaffold request validator
// set`).
package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
)

// ServiceCreateOptions parametriza ServiceCreate.
type ServiceCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "OrgMembership").
	Domain string
	// Verb é o nome do verbo do use case em PascalCase (ex.: "SelectOrg").
	// É convertido pra snake_case pro nome do pacote e pra lowerCamelCase pro
	// RegistryKey.
	Verb string
}

// ServiceCreate gera os três arquivos do use case em
// `internal/application/services/<snake_domain>/<snake_verb>/`:
//
//   - `request.go`: structs `Data` e `Request`, `NewRequest(data *Data)` e
//     `Validate()` (no-op enquanto não há validador configurado).
//   - `service.go`: `const RegistryKey`, struct `Service` embedando
//     `services.BaseService`, `NewService(logger, idCreator)`,
//     `Execute(ctx, *Request) error` e `GetResponse() (*Response, error)`.
//   - `response.go`: struct `Response` vazia.
//
// Retorna os caminhos relativos dos três arquivos gerados, em ordem fixa:
// request.go, service.go, response.go.
//
// Validações:
//   - Domain e Verb são PascalCase exportáveis.
//   - O arquivo do domain existe e contém a struct do Domain.
//   - A pasta do verbo dentro do domain ainda não existe.
//
// Falha sem mutar nada se qualquer validação falhar.
func ServiceCreate(opts ServiceCreateOptions) ([]string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return nil, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Verb) {
		return nil, fmt.Errorf("nome de verbo inválido (esperado PascalCase exportável): %q", opts.Verb)
	}

	if err := assertDomainStructExists(opts.Root, opts.Domain); err != nil {
		return nil, err
	}

	snakeDomain := ToSnake(opts.Domain)
	snakeVerb := ToSnake(opts.Verb)
	root := opts.Root
	if root == "" {
		root = "."
	}

	relDir := filepath.Join(servicesBasePath, snakeDomain, snakeVerb)
	absDir := filepath.Join(root, relDir)

	if _, err := os.Stat(absDir); err == nil {
		return nil, fmt.Errorf("service já existe: %s", relDir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", absDir, err)
	}

	imp, err := newImportPaths(root)
	if err != nil {
		return nil, err
	}
	requestSrc, err := buildRequestFile(snakeVerb, opts.Verb)
	if err != nil {
		return nil, err
	}
	serviceSrc, err := buildServiceFile(snakeVerb, opts.Verb, imp)
	if err != nil {
		return nil, err
	}
	responseSrc, err := buildResponseFile(snakeVerb, opts.Verb)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", absDir, err)
	}

	relRequest := filepath.Join(relDir, "request.go")
	relService := filepath.Join(relDir, "service.go")
	relResponse := filepath.Join(relDir, "response.go")

	files := []struct {
		rel string
		src []byte
	}{
		{relRequest, requestSrc},
		{relService, serviceSrc},
		{relResponse, responseSrc},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f.rel), f.src, 0o600); err != nil {
			return nil, fmt.Errorf("write %s: %w", f.rel, err)
		}
	}
	return []string{relRequest, relService, relResponse}, nil
}

// buildRequestFile monta `request.go` via AST puro com `Data` vazia e sem
// validator (variante padrão pós-`service create`). Sem package doc — fica
// em service.go pra evitar duplicação no godoc do pacote.
//
// Estrutura gerada:
//
//	package <pkg>
//
//	// Data agrega os campos de entrada validáveis do use case <Verb>.
//	type Data struct{}
//
//	// Request encapsula Data para o Execute do Service.
//	type Request struct {
//	    Data *Data
//	}
//
//	// NewRequest constrói o Request.
//	func NewRequest(data *Data) *Request {
//	    return &Request{Data: data}
//	}
//
//	// Validate valida Data. Sem validator configurado, retorna nil.
//	func (r *Request) Validate() error {
//	    return nil
//	}
func buildRequestFile(pkg, verb string) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-service-create-request")

	dataDecl := TypeDecl("Data", &ast.StructType{Fields: FieldList()})
	dataDecl.Doc = singleComment(fmt.Sprintf("// Data agrega os campos de entrada validáveis do use case %s.", verb))

	requestStruct := &ast.StructType{
		Fields: FieldList(Field("Data", StarOf(Ident("Data")))),
	}
	requestDecl := TypeDecl("Request", requestStruct)
	requestDecl.Doc = singleComment("// Request encapsula Data para o Execute do Service.")

	// Literal de retorno do constructor (return &Request{Data: data}).
	newReqReturn := &ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
		Type: Ident("Request"),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: Ident("Data"), Value: Ident("data")},
		},
	}}
	newRequestDecl := FuncDecl(
		nil,
		"NewRequest",
		FieldList(Field("data", StarOf(Ident("Data")))),
		FieldList(AnonField(StarOf(Ident("Request")))),
		[]ast.Stmt{ReturnStmt(newReqReturn)},
	)
	newRequestDecl.Doc = singleComment("// NewRequest constrói o Request.")

	validateDecl := FuncDecl(
		PointerReceiver("r", "Request"),
		"Validate",
		FieldList(),
		FieldList(AnonField(Ident("error"))),
		[]ast.Stmt{ReturnStmt(Ident("nil"))},
	)
	validateDecl.Doc = singleComment("// Validate valida Data. Sem validator configurado, retorna nil.")

	packagePos := padder.Take()
	padder.Gap(1)

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

	decls := []ast.Decl{dataDecl, requestDecl, newRequestDecl, validateDecl}
	file := &ast.File{
		Package: packagePos,
		Name:    Ident(pkg),
		Decls:   decls,
	}
	file.Comments = collectDocs(nil, decls)

	return formatFile(fset, file)
}

// buildServiceFile monta `service.go` via AST puro. Inclui o package doc
// (única ocorrência dentre os três arquivos).
//
// Estrutura gerada:
//
//	// Package <pkg> implementa o use case <Verb>.
//	package <pkg>
//
//	import (
//	    "context"
//
//	    "<module>/internal/application/services"
//	)
//
//	// RegistryKey identifica o *Service no pkg/registry.
//	const RegistryKey = "<lowerCamelVerb>Service"
//
//	// Service executa o use case <Verb>.
//	type Service struct {
//	    services.BaseService
//	    response *Response
//	}
//
//	// NewService constrói o Service com suas dependências.
//	func NewService(logger services.Logger, idCreator services.IdCreator) *Service { ... }
//
//	// Execute roda o use case <Verb>.
//	func (s *Service) Execute(_ context.Context, request *Request) error { ... }
//
//	// GetResponse devolve a resposta produzida pelo Execute.
//	func (s *Service) GetResponse() (*Response, error) { ... }
func buildServiceFile(pkg, verb string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-service-create-service")

	imports := ImportGroups(padder,
		[]string{"context"},
		[]string{imp.join(servicesImportSubpath)},
	)

	registryKey := ToLowerCamel(verb) + "Service"
	constDecl := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{Ident("RegistryKey")},
				Values: []ast.Expr{StrLit(registryKey)},
			},
		},
	}
	constDecl.Doc = singleComment("// RegistryKey identifica o *Service no pkg/registry.")

	serviceStruct := &ast.StructType{
		Fields: FieldList(
			AnonField(Sel("services", "BaseService")),
			Field("response", StarOf(Ident("Response"))),
		),
	}
	serviceDecl := TypeDecl("Service", serviceStruct)
	serviceDecl.Doc = singleComment(fmt.Sprintf("// Service executa o use case %s.", verb))

	baseServiceLit := &ast.CompositeLit{
		Type: Sel("services", "BaseService"),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: Ident("Logger"), Value: Ident("logger")},
			&ast.KeyValueExpr{Key: Ident("Ulid"), Value: Ident("idCreator")},
		},
	}
	serviceLit := &ast.CompositeLit{
		Type: Ident("Service"),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: Ident("BaseService"), Value: baseServiceLit},
		},
	}
	returnNew := &ast.UnaryExpr{Op: token.AND, X: serviceLit}

	newServiceDecl := FuncDecl(
		nil,
		"NewService",
		FieldList(
			Field("logger", Sel("services", "Logger")),
			Field("idCreator", Sel("services", "IdCreator")),
		),
		FieldList(AnonField(StarOf(Ident("Service")))),
		[]ast.Stmt{ReturnStmt(returnNew)},
	)
	newServiceDecl.Doc = singleComment("// NewService constrói o Service com suas dependências.")

	// Body do Execute: roda Validate, em caso de erro devolve um AppError de
	// categoria invalid; em sucesso atribui uma Response vazia ao campo interno
	// e retorna nil.
	validateCall := &ast.CallExpr{Fun: &ast.SelectorExpr{X: Ident("request"), Sel: Ident("Validate")}}
	validateIf := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{Ident("err")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{validateCall},
		},
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{
			ReturnStmt(&ast.CallExpr{
				Fun: Sel("services", "NewInvalid"),
				Args: []ast.Expr{
					&ast.CallExpr{Fun: &ast.SelectorExpr{X: Ident("err"), Sel: Ident("Error")}},
				},
			}),
		}},
	}
	respAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{&ast.SelectorExpr{X: Ident("s"), Sel: Ident("response")}},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{&ast.UnaryExpr{Op: token.AND, X: CompositeLit(Ident("Response"))}},
	}
	executeReturnNil := ReturnStmt(Ident("nil"))
	executeDecl := FuncDecl(
		PointerReceiver("s", "Service"),
		"Execute",
		FieldList(
			Field("_", Sel("context", "Context")),
			Field("request", StarOf(Ident("Request"))),
		),
		FieldList(AnonField(Ident("error"))),
		[]ast.Stmt{validateIf, respAssign, executeReturnNil},
	)
	executeDecl.Doc = singleComment(fmt.Sprintf("// Execute roda o use case %s.", verb))

	// Return do GetResponse: devolve s.response e nil (o erro já foi
	// sinalizado pelo Execute).
	getRespReturn := ReturnStmt(
		&ast.SelectorExpr{X: Ident("s"), Sel: Ident("response")},
		Ident("nil"),
	)
	getResponseDecl := FuncDecl(
		PointerReceiver("s", "Service"),
		"GetResponse",
		FieldList(),
		FieldList(
			AnonField(StarOf(Ident("Response"))),
			AnonField(Ident("error")),
		),
		[]ast.Stmt{getRespReturn},
	)
	getResponseDecl.Doc = singleComment("// GetResponse devolve a resposta produzida pelo Execute.")

	packageDoc := singleComment(fmt.Sprintf("// Package %s implementa o use case %s.", pkg, verb))

	decls := []ast.Decl{imports, constDecl, serviceDecl, newServiceDecl, executeDecl, getResponseDecl}
	stampedDecls := []ast.Decl{constDecl, serviceDecl, newServiceDecl, executeDecl, getResponseDecl}

	// Package doc primeiro (linha 1, antes do `package`).
	packageDoc.List[0].Slash = padder.Take()
	packagePos := padder.Take()
	padder.Gap(1)

	stampDocAndDecl(padder, constDecl)
	padder.Gap(1)
	stampDocAndDecl(padder, serviceDecl)
	padder.Gap(1)

	stampDocPositions(padder, newServiceDecl.Doc)
	newServiceDecl.Type.Func = padder.Take()
	newServiceDecl.Body.Lbrace = padder.Take()
	StampCompositeLit(padder, serviceLit)
	newServiceDecl.Body.Rbrace = padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, executeDecl.Doc)
	executeDecl.Type.Func = padder.Take()
	executeDecl.Body.Lbrace = padder.Take()
	// stmts internos: validateIf abre, return interno, fecha; depois o
	// assignment da response e o return nil final.
	validateIf.If = padder.Take()
	validateIf.Body.Lbrace = padder.Take()
	validateIf.Body.Rbrace = padder.Take()
	respAssign.TokPos = padder.Take()
	executeReturnNil.Return = padder.Take()
	executeDecl.Body.Rbrace = padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, getResponseDecl.Doc)
	getResponseDecl.Type.Func = padder.Take()
	getResponseDecl.Body.Lbrace = padder.Take()
	getRespReturn.Return = padder.Take()
	getResponseDecl.Body.Rbrace = padder.Take()

	file := &ast.File{
		Doc:     packageDoc,
		Package: packagePos,
		Name:    Ident(pkg),
		Decls:   decls,
	}
	file.Comments = collectDocs(packageDoc, stampedDecls)

	return formatFile(fset, file)
}

// buildResponseFile monta `response.go` via AST puro com apenas a struct
// `Response` vazia.
//
// Estrutura gerada:
//
//	package <pkg>
//
//	// Response agrega a saída do use case <Verb>.
//	type Response struct{}
func buildResponseFile(pkg, verb string) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-service-create-response")

	respDecl := TypeDecl("Response", &ast.StructType{Fields: FieldList()})
	respDecl.Doc = singleComment(fmt.Sprintf("// Response agrega a saída do use case %s.", verb))

	packagePos := padder.Take()
	padder.Gap(1)
	stampDocAndDecl(padder, respDecl)

	decls := []ast.Decl{respDecl}
	file := &ast.File{
		Package: packagePos,
		Name:    Ident(pkg),
		Decls:   decls,
	}
	file.Comments = collectDocs(nil, decls)

	return formatFile(fset, file)
}

// formatFile aplica format.Node + format.Source ao file e garante newline
// final. Mesma rotina usada pelos demais geradores do scaffold.
func formatFile(fset *token.FileSet, file *ast.File) ([]byte, error) {
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

// stampDocAndDecl stampa o doc comment (se houver) e a posição do GenDecl. Para
// GenDecl(TYPE) com struct/interface contendo campos, também stampa
// Opening/Closing pra ancorar as fronteiras em linhas separadas. Para structs/
// interfaces vazias usa a mesma Pos pros dois — o printer rendera `struct{}`/
// `interface{}` numa única linha. Sem stamping, Opening == NoPos faria o
// printer cair pra layout multi-line por default.
func stampDocAndDecl(p *LinePadder, gd *ast.GenDecl) {
	stampDocPositions(p, gd.Doc)
	gd.TokPos = p.Take()
	if gd.Tok != token.TYPE {
		return
	}
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		switch t := ts.Type.(type) {
		case *ast.StructType:
			if t.Fields == nil {
				continue
			}
			if len(t.Fields.List) > 0 {
				t.Fields.Opening = p.Take()
				t.Fields.Closing = p.Take()
			} else {
				pos := p.Take()
				t.Fields.Opening = pos
				t.Fields.Closing = pos
			}
		case *ast.InterfaceType:
			if t.Methods == nil {
				continue
			}
			if len(t.Methods.List) > 0 {
				t.Methods.Opening = p.Take()
				t.Methods.Closing = p.Take()
			} else {
				pos := p.Take()
				t.Methods.Opening = pos
				t.Methods.Closing = pos
			}
		}
	}
}
