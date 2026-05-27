// Package scaffold (área handler) constrói o adapter HTTP de um service via AST puro.
//
// A entrada `scaffold handler create <Domain> <Service>` gera o arquivo
// `cmd/http/handlers/<snake_domain>/<snake_service>/handler.go` com pacote
// `<snake_service>`, struct `<Pascal>Handler{ svc *<service>.Service }`,
// constructor `New<Pascal>Handler` que resolve o service do use case via
// `registry.Resolve` (eager — falha rápida no boot se a dep não estiver
// registrada) e método único `Handle(c echo.Context) error` que binda em
// `<service>.Input`, chama `Execute` e devolve o Output via
// `c.JSON(http.StatusOK, out)`.
//
// O padrão é 1:1 com o scaffold de service (NAVE-58): para cada use case há
// um service em `services/<d>/<s>/` e um handler em `handlers/<d>/<s>/`. O
// foco é estrutural: nenhum DTO local de request/response é gerado e o
// status code padrão é http.StatusOK — troca pra 201/204/etc. é manual.
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
)

const (
	handlersBasePath = "cmd/http/handlers"
	servicesBasePath = "internal/application/services"
)

const (
	netHTTPImportPath = "net/http"
	echoImportPath    = "github.com/labstack/echo/v4"
)

// HandlerCreateOptions parametriza HandlerCreate.
type HandlerCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "UsageRecord").
	Domain string
	// Service é o nome do use case em PascalCase (ex.: "Login", "Export").
	Service string
}

// HandlerCreate gera o arquivo `cmd/http/handlers/<snake_domain>/<snake_service>/handler.go`
// com o handler completo do use case (RegistryKey, struct, constructor e método
// `Handle` que invoca o service do use case). Retorna o caminho relativo
// à raiz.
//
// Validações (todas obrigatórias, falham sem mutar nada no disco):
//   - Domain e Service são PascalCase exportáveis.
//   - O arquivo do domínio existe e contém a struct <Domain>.
//   - O arquivo do service existe e contém `const RegistryKey` e
//     `func NewService` (mesma checagem usada por NAVE-60).
//   - A pasta do handler ainda não existe.
func HandlerCreate(opts HandlerCreateOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Service) {
		return "", fmt.Errorf("nome de service inválido (esperado PascalCase exportável): %q", opts.Service)
	}

	snakeDomain := ToSnake(opts.Domain)
	snakeService := ToSnake(opts.Service)

	if err := assertDomainStructExists(opts.Root, opts.Domain); err != nil {
		return "", err
	}
	if err := assertServiceExists(opts.Root, snakeDomain, snakeService); err != nil {
		return "", err
	}

	withValidator, err := requestUsesValidator(opts.Root, snakeDomain, snakeService)
	if err != nil {
		return "", err
	}

	root := opts.Root
	if root == "" {
		root = "."
	}
	relDir := filepath.Join(handlersBasePath, snakeDomain, snakeService)
	relFile := filepath.Join(relDir, "handler.go")
	absDir := filepath.Join(root, relDir)
	absFile := filepath.Join(root, relFile)

	if _, err := os.Stat(absDir); err == nil {
		return "", fmt.Errorf("handler já existe: %s", relDir)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", absDir, err)
	}

	imp, err := newImportPaths(root)
	if err != nil {
		return "", err
	}
	src, err := buildHandlerFile(snakeDomain, snakeService, opts.Service, withValidator, imp)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", absDir, err)
	}
	if err := os.WriteFile(absFile, src, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", absFile, err)
	}
	return relFile, nil
}

func assertDomainStructExists(root, domain string) error {
	if root == "" {
		root = "."
	}
	snake := ToSnake(domain)
	relFile := filepath.Join(domainBasePath, snake, snake+".go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated domain name, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("ler domínio %s: %w", relFile, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	if !hasStruct(file, domain) {
		return fmt.Errorf("em %s: struct %s não encontrada", relFile, domain)
	}
	return nil
}

// assertServiceExists confirma que o arquivo do service existe e contém os
// símbolos referenciados pelo handler gerado (`RegistryKey`, `NewService`).
// Não checa assinatura — o build é o safety net.
func assertServiceExists(root, snakeDomain, snakeService string) error {
	if root == "" {
		root = "."
	}
	relFile := filepath.Join(servicesBasePath, snakeDomain, snakeService, "service.go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("ler service %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	if !hasConstNamed(file, "RegistryKey") {
		return fmt.Errorf("em %s: const RegistryKey não encontrado", relFile)
	}
	if !hasFreeFuncNamed(file, "NewService") {
		return fmt.Errorf("em %s: func NewService não encontrado", relFile)
	}
	return nil
}

func hasStruct(file *ast.File, typeName string) bool {
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
			if _, ok := ts.Type.(*ast.StructType); ok {
				return true
			}
		}
	}
	return false
}

func hasConstNamed(file *ast.File, constName string) bool {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, n := range vs.Names {
				if n.Name == constName {
					return true
				}
			}
		}
	}
	return false
}

func hasFreeFuncNamed(file *ast.File, funcName string) bool {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name == nil {
			continue
		}
		if fd.Name.Name == funcName {
			return true
		}
	}
	return false
}

// requestUsesValidator inspeciona o request.go do service pra decidir se
// NewRequest aceita um *services.Validator (caso `request validator set`).
// Falha se o request.go não existe (gerado pelo scaffold service create).
func requestUsesValidator(root, snakeDomain, snakeService string) (bool, error) {
	if root == "" {
		root = "."
	}
	relFile := filepath.Join(servicesBasePath, snakeDomain, snakeService, "request.go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return false, fmt.Errorf("ler request %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", relFile, err)
	}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name == nil || fd.Name.Name != "NewRequest" {
			continue
		}
		count := 0
		if fd.Type.Params != nil {
			for _, f := range fd.Type.Params.List {
				count += len(f.Names)
				if len(f.Names) == 0 {
					count++
				}
			}
		}
		return count >= 2, nil
	}
	return false, fmt.Errorf("em %s: func NewRequest não encontrada", relFile)
}

// buildHandlerFile monta, via AST puro, o arquivo Go do handler 1:1.
//
// Quando withValidator=true a struct Handler ganha o campo
// `validator services.Validator`, o ctor resolve `validator.RegistryKey` do
// registry e o Handle passa o validator ao `NewRequest`.
func buildHandlerFile(snakeDomain, snakeService, service string, withValidator bool, imp importPaths) ([]byte, error) {
	handlerType := service + "Handler"
	servicePkg := snakeService

	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-handler-create")

	mainImports := []string{
		imp.join(httperrImportSubpath),
		imp.join(servicesImportSubpath + "/" + snakeDomain + "/" + snakeService),
		imp.join(registryImportSubpath),
	}
	if withValidator {
		// `validator` no ctor é só o nome do pacote (referenciado em
		// `validator.RegistryKey`). O resolve cai numa variável local
		// chamada `valSvc` pra evitar shadowing do pacote dentro do escopo
		// onde ainda precisamos do RegistryKey.
		mainImports = append(mainImports,
			imp.join(servicesImportSubpath),
			imp.join(validatorImportSubpath),
		)
	}

	imports := ImportGroups(padder,
		[]string{netHTTPImportPath},
		mainImports,
		[]string{echoImportPath},
	)

	registryKey := ToLowerCamel(service) + "Handler"
	constDecl := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{Ident("RegistryKey")},
				Values: []ast.Expr{StrLit(registryKey)},
			},
		},
	}
	constDecl.Doc = singleComment(fmt.Sprintf("// RegistryKey identifica o *%s no pkg/registry.", handlerType))

	structFields := []*ast.Field{Field("svc", StarOf(Sel(servicePkg, "Service")))}
	if withValidator {
		structFields = append(structFields, Field("validator", Sel("services", "Validator")))
	}
	structType := &ast.StructType{Fields: &ast.FieldList{List: structFields}}
	structDecl := TypeDecl(handlerType, structType)
	structDecl.Doc = singleComment(fmt.Sprintf("// %s atende o use case %s. Mantém as deps já resolvidas pelo New.", handlerType, service))

	// svc := registry.Resolve[*<servicePkg>.Service](reg, <servicePkg>.RegistryKey)
	resolveCall := &ast.CallExpr{
		Fun: IndexExpr(
			Sel("registry", "Resolve"),
			StarOf(Sel(servicePkg, "Service")),
		),
		Args: []ast.Expr{
			Ident("reg"),
			Sel(servicePkg, "RegistryKey"),
		},
	}
	svcLhs := Ident("svc")
	svcAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{svcLhs},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{resolveCall},
	}
	// return &<Handler>{svc: svc[, validator: valSvc]}
	ctorElts := []ast.Expr{
		&ast.KeyValueExpr{Key: Ident("svc"), Value: Ident("svc")},
	}
	ctorStmts := []ast.Stmt{svcAssign}
	var valSvcAssign *ast.AssignStmt
	var valSvcLhs *ast.Ident
	if withValidator {
		// Resolve do services.Validator no registry para alimentar o campo
		// `validator` do handler.
		valResolve := &ast.CallExpr{
			Fun: IndexExpr(
				Sel("registry", "Resolve"),
				Sel("services", "Validator"),
			),
			Args: []ast.Expr{
				Ident("reg"),
				Sel("validator", "RegistryKey"),
			},
		}
		valSvcLhs = Ident("valSvc")
		valSvcAssign = &ast.AssignStmt{
			Lhs: []ast.Expr{valSvcLhs},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{valResolve},
		}
		ctorStmts = append(ctorStmts, valSvcAssign)
		ctorElts = append(ctorElts, &ast.KeyValueExpr{
			Key:   Ident("validator"),
			Value: Ident("valSvc"),
		})
	}
	ctorReturnExpr := &ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
		Type: Ident(handlerType),
		Elts: ctorElts,
	}}
	ctorReturnStmt := ReturnStmt(ctorReturnExpr)
	ctorStmts = append(ctorStmts, ctorReturnStmt)
	ctorDecl := FuncDecl(
		nil,
		"New"+handlerType,
		FieldList(Field("reg", StarOf(Sel("registry", "Registry")))),
		FieldList(AnonField(StarOf(Ident(handlerType)))),
		ctorStmts,
	)
	ctorDecl.Doc = singleComment(fmt.Sprintf("// New%s resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).", handlerType))

	handleDecl := buildHandleMethod(handlerType, service, servicePkg, withValidator)

	packageDoc := singleComment(fmt.Sprintf("// Package %s expõe o handler HTTP do use case %s.", servicePkg, service))

	// Layout: doc do package, import block, const, struct, constructor, Handle.
	packageDoc.List[0].Slash = padder.Take()
	packagePos := padder.Take()
	padder.Gap(1)

	stampDeclWithDoc(padder, constDecl)
	padder.Gap(1)
	stampDeclWithDoc(padder, structDecl)
	padder.Gap(1)
	// Constructor stampado com posições explícitas nos statements internos: o
	// resolveCall tem muitos tokens sem Pos, então sem isso a doc do Handle
	// (próxima Decl) pode acabar encaixada entre tokens do registry.Resolve.
	// Ao stampar svcAssign e ctorReturnStmt, o printer fixa o lugar de cada
	// statement e o comentário fica fora do corpo.
	stampDocPositions(padder, ctorDecl.Doc)
	ctorDecl.Type.Func = padder.Take()
	ctorDecl.Body.Lbrace = padder.Take()
	svcLhs.NamePos = padder.Take()
	svcAssign.TokPos = svcLhs.NamePos
	if withValidator {
		valSvcLhs.NamePos = padder.Take()
		valSvcAssign.TokPos = valSvcLhs.NamePos
	}
	ctorReturnStmt.Return = padder.Take()
	ctorDecl.Body.Rbrace = padder.Take()
	padder.Gap(1)
	stampDeclWithDoc(padder, handleDecl)

	decls := []ast.Decl{imports, constDecl, structDecl, ctorDecl, handleDecl}
	file := &ast.File{
		Doc:     packageDoc,
		Package: packagePos,
		Name:    Ident(servicePkg),
		Decls:   decls,
	}
	file.Comments = collectDocs(packageDoc, decls)

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

// buildHandleMethod monta, via AST puro:
//
//	// Handle executa o use case <Service>.
//	func (h *<Pascal>Handler) Handle(c echo.Context) error {
//	    var data <servicePkg>.Data
//	    if err := c.Bind(&data); err != nil {
//	        return httperr.RespondBadRequest(c, err.Error())
//	    }
//	    req := <servicePkg>.NewRequest(&data[, h.validator])
//	    if err := h.svc.Execute(c.Request().Context(), req); err != nil {
//	        return httperr.Respond(c, err)
//	    }
//	    out, _ := h.svc.GetResponse()
//	    return c.JSON(http.StatusOK, out)
//	}
//
// Quando withValidator=true o NewRequest recebe o validator resolvido pelo
// ctor (`h.validator`) como segundo argumento.
func buildHandleMethod(handlerType, service, servicePkg string, withValidator bool) *ast.FuncDecl {
	// Declaração da variável de entrada com o tipo Data do request.
	dataDecl := &ast.DeclStmt{Decl: &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{Ident("data")},
				Type:  Sel(servicePkg, "Data"),
			},
		},
	}}

	// Bind do payload no Data; falha vira BadRequest via httperr.
	bindIf := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{Ident("err")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.CallExpr{
				Fun: Sel("c", "Bind"),
				Args: []ast.Expr{
					&ast.UnaryExpr{Op: token.AND, X: Ident("data")},
				},
			}},
		},
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{
			ReturnStmt(&ast.CallExpr{
				Fun: Sel("httperr", "RespondBadRequest"),
				Args: []ast.Expr{
					Ident("c"),
					&ast.CallExpr{Fun: &ast.SelectorExpr{
						X:   Ident("err"),
						Sel: Ident("Error"),
					}},
				},
			}),
		}},
	}

	// req := <servicePkg>.NewRequest(&data[, h.validator])
	newReqArgs := []ast.Expr{&ast.UnaryExpr{Op: token.AND, X: Ident("data")}}
	if withValidator {
		newReqArgs = append(newReqArgs, Sel("h", "validator"))
	}
	reqAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("req")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{
			Fun:  Sel(servicePkg, "NewRequest"),
			Args: newReqArgs,
		}},
	}

	// Encadeamento usado como primeiro argumento do Execute.
	ctxCall := &ast.CallExpr{Fun: &ast.SelectorExpr{
		X:   &ast.CallExpr{Fun: Sel("c", "Request")},
		Sel: Ident("Context"),
	}}

	// Execução do service: Execute retorna error. Falha vira a resposta de
	// erro padrão via httperr.Respond (mapeia o AppError pra HTTP status).
	executeIf := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{Ident("err")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   Sel("h", "svc"),
					Sel: Ident("Execute"),
				},
				Args: []ast.Expr{ctxCall, Ident("req")},
			}},
		},
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{
			ReturnStmt(&ast.CallExpr{
				Fun:  Sel("httperr", "Respond"),
				Args: []ast.Expr{Ident("c"), Ident("err")},
			}),
		}},
	}

	// Recupera a resposta produzida pelo Execute; o erro já foi tratado acima,
	// então é descartado aqui.
	getRespAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("out"), Ident("_")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   Sel("h", "svc"),
				Sel: Ident("GetResponse"),
			},
		}},
	}

	// Resposta default em sucesso — status 200 fixo, trocar é manual.
	returnJSON := ReturnStmt(&ast.CallExpr{
		Fun:  Sel("c", "JSON"),
		Args: []ast.Expr{Sel("http", "StatusOK"), Ident("out")},
	})

	handle := FuncDecl(
		PointerReceiver("h", handlerType),
		"Handle",
		FieldList(Field("c", Sel("echo", "Context"))),
		FieldList(AnonField(Ident("error"))),
		[]ast.Stmt{dataDecl, bindIf, reqAssign, executeIf, getRespAssign, returnJSON},
	)
	handle.Doc = singleComment(fmt.Sprintf("// Handle executa o use case %s.", service))
	return handle
}

func singleComment(text string) *ast.CommentGroup {
	return &ast.CommentGroup{List: []*ast.Comment{{Text: text}}}
}

// stampDeclWithDoc reserva linhas para o doc comment (se houver) e a Pos
// principal do Decl. Para GenDecl(TYPE) com struct contendo campos também
// stampa Opening/Closing das Fields para forçar layout multi-line. Para
// FuncDecl stampa Func, Lbrace e Rbrace.
func stampDeclWithDoc(p *LinePadder, d ast.Decl) {
	switch x := d.(type) {
	case *ast.GenDecl:
		stampDocPositions(p, x.Doc)
		x.TokPos = p.Take()
		if x.Tok != token.TYPE {
			return
		}
		for _, spec := range x.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			if len(st.Fields.List) > 0 {
				st.Fields.Opening = p.Take()
				st.Fields.Closing = p.Take()
			} else {
				pos := p.Take()
				st.Fields.Opening = pos
				st.Fields.Closing = pos
			}
		}
	case *ast.FuncDecl:
		stampDocPositions(p, x.Doc)
		x.Type.Func = p.Take()
		if x.Body != nil {
			x.Body.Lbrace = p.Take()
			x.Body.Rbrace = p.Take()
		}
	}
}

// stampDocPositions reserva uma linha por *ast.Comment do grupo, garantindo
// que o doc fique acima do decl no output do printer.
func stampDocPositions(p *LinePadder, doc *ast.CommentGroup) {
	if doc == nil {
		return
	}
	for _, c := range doc.List {
		c.Slash = p.Take()
	}
}

func collectDocs(pkgDoc *ast.CommentGroup, decls []ast.Decl) []*ast.CommentGroup {
	out := make([]*ast.CommentGroup, 0, len(decls)+1)
	if pkgDoc != nil {
		out = append(out, pkgDoc)
	}
	for _, d := range decls {
		switch x := d.(type) {
		case *ast.GenDecl:
			if x.Doc != nil {
				out = append(out, x.Doc)
			}
		case *ast.FuncDecl:
			if x.Doc != nil {
				out = append(out, x.Doc)
			}
		}
	}
	return out
}
