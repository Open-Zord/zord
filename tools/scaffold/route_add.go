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

// allowedMethods enumera os verbos HTTP aceitos pelo `route add`.
var allowedMethods = map[string]struct{}{
	"GET": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {},
}

// RouteAddOptions parametriza RouteAdd.
type RouteAddOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o domain em PascalCase. Determina o arquivo de rotas alvo
	// (`cmd/http/routes/<snake>.go`), o struct `<Pascal>Route` e o segmento
	// inicial do path da rota.
	Domain string
	// Service é o use case em PascalCase. Determina o nome do campo do
	// handler, o package importado e o path default.
	Service string
	// Method é o verbo HTTP (GET|POST|PUT|PATCH|DELETE). Case-insensitive,
	// normalizado pra uppercase no output.
	Method string
	// Path sobrescreve o path default `/<kebab-service>`.
	Path string
	// Public registra a rota em DeclarePublicRoutes em vez de
	// DeclarePrivateRoutes.
	Public bool
}

// RouteAdd altera `cmd/http/routes/<snake_domain>.go` em quatro pontos para
// registrar uma rota que aponta para o handler 1:1 do service informado.
// Retorna o caminho relativo à raiz. Pontos alterados:
//  1. Campo `<lowerCamel>Handler *<snake_service>.<Pascal>Handler` na struct.
//  2. Atribuição `<lowerCamel>Handler: registry.Resolve[*<snake_service>.<Pascal>Handler](reg, <snake_service>.RegistryKey)`
//     no CompositeLit retornado pelo constructor.
//  3. Linha `g.<METHOD>(...)` em DeclarePrivateRoutes ou DeclarePublicRoutes
//     (conforme `Public`).
//  4. Import do pacote do handler.
//
// O constructor `New<Pascal>Route(reg *registry.Registry)` mantém
// assinatura imutável — handlers são resolvidos do registry internamente
// (eager).
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain e Service são PascalCase exportáveis.
//   - Method ∈ {GET, POST, PUT, PATCH, DELETE} (case-insensitive).
//   - O arquivo de rotas existe e contém a struct `<Pascal>Route`, o
//     construtor `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route`
//     e ambas as funções `DeclarePrivateRoutes` e `DeclarePublicRoutes`.
//     Constructor com assinatura diferente (parâmetros extras, ausência do
//     `reg`, etc.) falha — Route hand-editada.
//   - O service existe (`services/<snake_domain>/<snake_service>/service.go`
//     com `const RegistryKey` + `func NewService`).
//   - O handler 1:1 existe (`handlers/<snake_domain>/<snake_service>/handler.go`
//     com struct `<Pascal>Handler` e método `Handle(echo.Context) error`).
//   - A rota ainda não foi registrada: o campo `<lowerCamel>Handler` não
//     existe na struct e nenhuma chamada `r.<lowerCamel>Handler.Handle`
//     aparece em DeclarePrivateRoutes/DeclarePublicRoutes.
func RouteAdd(opts RouteAddOptions) (string, error) {
	plan, err := planAdd(opts)
	if err != nil {
		return "", err
	}

	if err := assertServiceExists(plan.root, plan.snakeDomain, plan.snakeService); err != nil {
		return "", err
	}
	if err := assertHandlerHasHandleMethod(plan.root, plan.snakeDomain, plan.snakeService, opts.Service); err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	src, err := os.ReadFile(plan.absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", plan.relFile, err)
	}
	file, err := parser.ParseFile(fset, plan.absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", plan.relFile, err)
	}

	parts, err := findRouteParts(file, plan.routeType)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	if err := assertNotAlreadyRegistered(plan, parts); err != nil {
		return "", err
	}

	applyAddPatches(fset, file, parts, plan, opts.Public)

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// addPlan agrupa nomes derivados e caminhos pra evitar repetir a derivação
// nos vários passos do RouteAdd.
type addPlan struct {
	root              string
	relFile           string
	absFile           string
	snakeDomain       string
	snakeService      string
	routeType         string
	fieldName         string
	handlerType       string
	method            string
	path              string
	handlerImportPath string
}

func planAdd(opts RouteAddOptions) (addPlan, error) {
	var plan addPlan
	if !IsValidExportedIdent(opts.Domain) {
		return plan, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Service) {
		return plan, fmt.Errorf("nome de service inválido (esperado PascalCase exportável): %q", opts.Service)
	}
	method, err := normalizeMethod(opts.Method)
	if err != nil {
		return plan, err
	}

	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	plan.snakeDomain = ToSnake(opts.Domain)
	plan.snakeService = ToSnake(opts.Service)
	plan.routeType = opts.Domain + "Route"
	plan.fieldName = ToLowerCamel(opts.Service) + "Handler"
	plan.handlerType = opts.Service + "Handler"
	plan.method = method
	plan.relFile = filepath.Join(routesBasePath, plan.snakeDomain+".go")
	plan.absFile = filepath.Join(plan.root, plan.relFile)

	plan.path = opts.Path
	if plan.path == "" {
		plan.path = "/" + strings.ReplaceAll(plan.snakeService, "_", "-")
	}

	imp, err := newImportPaths(plan.root)
	if err != nil {
		return plan, err
	}
	plan.handlerImportPath = imp.join(handlersImportSubpath + "/" + plan.snakeDomain + "/" + plan.snakeService)
	return plan, nil
}

func assertNotAlreadyRegistered(plan addPlan, parts routeParts) error {
	if hasFieldNamed(parts.structType, plan.fieldName) {
		return fmt.Errorf("em %s: rota já registrada — campo %s.%s existe", plan.relFile, plan.routeType, plan.fieldName)
	}
	if hasHandlerCall(parts.declarePrivate, plan.fieldName) || hasHandlerCall(parts.declarePublic, plan.fieldName) {
		return fmt.Errorf("em %s: rota já registrada — r.%s.Handle existe em Declare*Routes", plan.relFile, plan.fieldName)
	}
	return nil
}

func applyAddPatches(fset *token.FileSet, file *ast.File, parts routeParts, plan addPlan, public bool) {
	fieldType := StarOf(Sel(plan.snakeService, plan.handlerType))
	appendStructField(parts.structType, plan.fieldName, fieldType)
	appendCtorResolve(fset, parts.ctor, plan.fieldName, plan.snakeService, plan.handlerType)

	target := parts.declarePrivate
	if public {
		target = parts.declarePublic
	}
	appendRouteCall(target, plan.method, plan.snakeDomain, plan.path, plan.fieldName)

	astutil.AddImport(fset, file, plan.handlerImportPath)
}

// routeParts agrupa os nós AST relevantes para patches do `route add`.
type routeParts struct {
	structType     *ast.StructType
	ctor           *ast.FuncDecl
	declarePrivate *ast.FuncDecl
	declarePublic  *ast.FuncDecl
}

// findRouteParts localiza, no arquivo da Route, a struct `<Pascal>Route`, o
// construtor `New<Pascal>Route` e as duas funções `Declare*Routes`. Retorna
// erro se qualquer um faltar — o `route add` exige o esqueleto canônico
// emitido por `route create`.
func findRouteParts(file *ast.File, routeType string) (routeParts, error) {
	var parts routeParts
	ctorName := "New" + routeType
	for _, decl := range file.Decls {
		if err := collectRoutePart(decl, routeType, ctorName, &parts); err != nil {
			return parts, err
		}
	}
	return parts, validateRouteParts(parts, routeType, ctorName)
}

func collectRoutePart(decl ast.Decl, routeType, ctorName string, parts *routeParts) error {
	switch d := decl.(type) {
	case *ast.GenDecl:
		st, err := matchRouteStruct(d, routeType)
		if err != nil {
			return err
		}
		if st != nil {
			parts.structType = st
		}
	case *ast.FuncDecl:
		matchRouteFunc(d, routeType, ctorName, parts)
	}
	return nil
}

func matchRouteStruct(gd *ast.GenDecl, routeType string) (*ast.StructType, error) {
	if gd.Tok != token.TYPE {
		return nil, nil
	}
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Name.Name != routeType {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return nil, fmt.Errorf("tipo %s não é struct", routeType)
		}
		if st.Fields == nil {
			st.Fields = &ast.FieldList{}
		}
		return st, nil
	}
	return nil, nil
}

func matchRouteFunc(fd *ast.FuncDecl, routeType, ctorName string, parts *routeParts) {
	if fd.Name == nil {
		return
	}
	if fd.Recv == nil {
		if fd.Name.Name == ctorName {
			parts.ctor = fd
		}
		return
	}
	if !receiverMatchesPointer(fd.Recv, routeType) {
		return
	}
	switch fd.Name.Name {
	case "DeclarePrivateRoutes":
		parts.declarePrivate = fd
	case "DeclarePublicRoutes":
		parts.declarePublic = fd
	}
}

func validateRouteParts(parts routeParts, routeType, ctorName string) error {
	if parts.structType == nil {
		return fmt.Errorf("struct %s não encontrada", routeType)
	}
	if parts.ctor == nil {
		return fmt.Errorf("construtor %s não encontrado", ctorName)
	}
	if err := assertCtorSignature(parts.ctor, ctorName); err != nil {
		return err
	}
	if parts.declarePrivate == nil {
		return fmt.Errorf("método (r *%s) DeclarePrivateRoutes não encontrado", routeType)
	}
	if parts.declarePublic == nil {
		return fmt.Errorf("método (r *%s) DeclarePublicRoutes não encontrado", routeType)
	}
	return nil
}

// assertCtorSignature exige a assinatura canônica
// `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route` — single source
// of truth do shape emitido por `route create` (NAVE-74). Qualquer divergência
// (parâmetro extra, tipo errado, ausência de `reg`) sinaliza Route
// hand-editada e impede o `route add` de gerar código incoerente.
func assertCtorSignature(ctor *ast.FuncDecl, ctorName string) error {
	params := ctor.Type.Params
	wrap := func(msg string) error {
		return fmt.Errorf("construtor %s %s — esperado assinatura canônica (reg *registry.Registry)", ctorName, msg)
	}
	if params == nil || len(params.List) != 1 {
		return wrap("deve ter exatamente um parâmetro")
	}
	f := params.List[0]
	if len(f.Names) != 1 || f.Names[0].Name != "reg" {
		return wrap("primeiro parâmetro deve se chamar `reg`")
	}
	star, ok := f.Type.(*ast.StarExpr)
	if !ok {
		return wrap("`reg` deve ser ponteiro")
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return wrap("`reg` deve ser `*registry.Registry`")
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "registry" || sel.Sel == nil || sel.Sel.Name != "Registry" {
		return wrap("`reg` deve ser `*registry.Registry`")
	}
	return nil
}

func receiverMatchesPointer(recv *ast.FieldList, typeName string) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	star, ok := recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := star.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == typeName
}

func hasFieldNamed(st *ast.StructType, fieldName string) bool {
	if st == nil || st.Fields == nil {
		return false
	}
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == fieldName {
				return true
			}
		}
	}
	return false
}

// hasHandlerCall procura, no corpo da função, qualquer expressão
// `r.<fieldName>.Handle` (selector chain de duas camadas a partir de `r`).
func hasHandlerCall(fd *ast.FuncDecl, fieldName string) bool {
	if fd == nil || fd.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		outer, ok := n.(*ast.SelectorExpr)
		if !ok || outer.Sel == nil || outer.Sel.Name != "Handle" {
			return true
		}
		inner, ok := outer.X.(*ast.SelectorExpr)
		if !ok || inner.Sel == nil || inner.Sel.Name != fieldName {
			return true
		}
		id, ok := inner.X.(*ast.Ident)
		if !ok || id.Name != "r" {
			return true
		}
		found = true
		return false
	})
	return found
}

func appendStructField(st *ast.StructType, fieldName string, fieldType ast.Expr) {
	st.Fields.List = append(st.Fields.List, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(fieldName)},
		Type:  fieldType,
	})
}

// appendCtorResolve anexa, ao CompositeLit retornado pelo constructor, a
// atribuição
//
//	<fieldName>: registry.Resolve[*<snakeService>.<handlerType>](reg, <snakeService>.RegistryKey)
//
// O constructor sempre recebe `reg *registry.Registry` (shape canônico —
// NAVE-74), então nenhum parâmetro é adicionado: o `reg` já existe no
// escopo. Cada `route add` apenas pluga a resolução do handler no
// CompositeLit, mantendo a assinatura do constructor estável.
//
// Após anexar, força layout multi-line via StampCompositeLit:
// chamadas `registry.Resolve[...]` são longas e cada uma numa linha mantém
// o constructor legível (gofmt sozinho não quebra essa expressão).
func appendCtorResolve(fset *token.FileSet, ctor *ast.FuncDecl, fieldName, snakeService, handlerType string) {
	if ctor.Body == nil {
		return
	}
	for _, stmt := range ctor.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		ue, ok := ret.Results[0].(*ast.UnaryExpr)
		if !ok || ue.Op != token.AND {
			continue
		}
		cl, ok := ue.X.(*ast.CompositeLit)
		if !ok {
			continue
		}
		resolveCall := &ast.CallExpr{
			Fun: IndexExpr(
				Sel("registry", "Resolve"),
				StarOf(Sel(snakeService, handlerType)),
			),
			Args: []ast.Expr{
				ast.NewIdent("reg"),
				Sel(snakeService, "RegistryKey"),
			},
		}
		cl.Elts = append(cl.Elts, &ast.KeyValueExpr{
			Key:   ast.NewIdent(fieldName),
			Value: resolveCall,
		})
		padder := NewLinePadder(fset, "scaffold-route-add-ctorlit")
		stampCompositeLitDeep(padder, cl)
		// Restampar o Rbrace do body do constructor logo após o Rbrace do
		// CompositeLit elimina a linha em branco que apareceria por causa
		// do gap entre a Pos do Rbrace original (do arquivo parseado) e o
		// novo Rbrace do CompositeLit (na padder file sintética).
		ctor.Body.Rbrace = padder.Take()
		return
	}
}

// appendRouteCall anexa ao corpo da Declare*Routes a chamada:
//
//	g.<METHOD>("/"+prefix+"/<snake_domain>"+"<path>", r.<fieldName>.Handle)
func appendRouteCall(fd *ast.FuncDecl, method, snakeDomain, path, fieldName string) {
	pathExpr := &ast.BinaryExpr{
		Op: token.ADD,
		X: &ast.BinaryExpr{
			Op: token.ADD,
			X: &ast.BinaryExpr{
				Op: token.ADD,
				X:  StrLit("/"),
				Y:  ast.NewIdent("prefix"),
			},
			Y: StrLit("/" + snakeDomain),
		},
		Y: StrLit(path),
	}
	handlerRef := &ast.SelectorExpr{
		X: &ast.SelectorExpr{
			X:   ast.NewIdent("r"),
			Sel: ast.NewIdent(fieldName),
		},
		Sel: ast.NewIdent("Handle"),
	}
	call := &ast.ExprStmt{X: &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("g"),
			Sel: ast.NewIdent(method),
		},
		Args: []ast.Expr{pathExpr, handlerRef},
	}}
	fd.Body.List = append(fd.Body.List, call)
}

func normalizeMethod(m string) (string, error) {
	up := strings.ToUpper(strings.TrimSpace(m))
	if up == "" {
		return "", fmt.Errorf("--method é obrigatório (GET|POST|PUT|PATCH|DELETE)")
	}
	if _, ok := allowedMethods[up]; !ok {
		return "", fmt.Errorf("--method inválido %q (esperado GET|POST|PUT|PATCH|DELETE)", m)
	}
	return up, nil
}

// assertHandlerHasHandleMethod confirma que o handler existe com a struct
// `<Service>Handler` e o método `(*<Service>Handler).Handle(echo.Context) error`.
// Distinta de assertHandlerExists (register_handler.go), que checa
// `const RegistryKey` + `func New<Pascal>Handler` — o `route add` precisa do
// método Handle, o `handler register` precisa do constructor.
func assertHandlerHasHandleMethod(root, snakeDomain, snakeService, service string) error {
	relFile := filepath.Join(handlersBasePath, snakeDomain, snakeService, "handler.go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated identifiers, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("ler handler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	handlerType := service + "Handler"
	if !hasStruct(file, handlerType) {
		return fmt.Errorf("em %s: struct %s não encontrada", relFile, handlerType)
	}
	if !hasHandleMethod(file, handlerType) {
		return fmt.Errorf("em %s: método (*%s).Handle(echo.Context) error não encontrado", relFile, handlerType)
	}
	return nil
}

// hasHandleMethod confirma que o handler tem o método único esperado:
// `func (h *<Pascal>Handler) Handle(c echo.Context) error`. Checa apenas o
// receiver e o nome; assinatura completa cai pro safety net do `go build`.
func hasHandleMethod(file *ast.File, handlerType string) bool {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name == nil || fd.Name.Name != "Handle" {
			continue
		}
		if receiverMatchesPointer(fd.Recv, handlerType) {
			return true
		}
	}
	return false
}
