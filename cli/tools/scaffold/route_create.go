// Package scaffold (área route) entrega o último elo da cadeia do scaffold backend: o arquivo
// de rotas HTTP por domain. `route create` gera o esqueleto vazio
// (`cmd/http/routes/<snake>.go` com struct, construtor recebendo apenas
// `reg *registry.Registry`, e funções `DeclarePrivateRoutes` /
// `DeclarePublicRoutes`); `route add` altera esse arquivo via AST puro,
// anexando o campo do handler 1:1, a atribuição com `registry.Resolve[...]`
// no CompositeLit do construtor e a linha de registro em uma das funções
// `Declare*`. O constructor da Route mantém assinatura imutável
// `(reg *registry.Registry)` sem variar com o número de services — cada
// Route resolve os próprios handlers do registry internamente (eager). O
// par fecha a cadeia `domain → service → handler → route` (NAVE-56,
// fatias 6 e — novo shape — NAVE-74).
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

const routesBasePath = "cmd/http/routes"

// RouteCreateOptions parametriza RouteCreate.
type RouteCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "UsageRecord").
	Domain string
}

// RouteCreate gera o arquivo `cmd/http/routes/<snake_domain>.go` com o esqueleto
// vazio de uma Route: struct `<Pascal>Route` sem campos, construtor
// `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route` retornando
// `&<Pascal>Route{}`, e funções `DeclarePrivateRoutes` /
// `DeclarePublicRoutes` com corpos vazios. Retorna o caminho relativo à
// raiz. O parâmetro `reg` permanece sem uso inicial; cada `route add`
// posterior introduz uma atribuição `<lowerCamel>Handler: registry.Resolve[...]`
// no CompositeLit do retorno do constructor.
//
// Validações (todas obrigatórias, falham sem mutar nada no disco):
//   - Domain é PascalCase exportável.
//   - O arquivo do domínio existe e contém a struct <Domain>.
//   - O arquivo da Route ainda não existe.
func RouteCreate(opts RouteCreateOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if err := assertDomainStructExists(opts.Root, opts.Domain); err != nil {
		return "", err
	}

	snakeDomain := ToSnake(opts.Domain)
	root := opts.Root
	if root == "" {
		root = "."
	}

	relFile := filepath.Join(routesBasePath, snakeDomain+".go")
	absFile := filepath.Join(root, relFile)

	if _, err := os.Stat(absFile); err == nil {
		return "", fmt.Errorf("arquivo de rotas já existe: %s", relFile)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", absFile, err)
	}

	imp, err := newImportPaths(root)
	if err != nil {
		return "", err
	}
	src, err := buildRouteFile(opts.Domain, imp)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(absFile, src, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", absFile, err)
	}
	return relFile, nil
}

// buildRouteFile monta, via AST puro, o arquivo de rotas vazio do domain.
// O constructor recebe `reg *registry.Registry` para que cada `route add`
// possa anexar `<lowerCamel>Handler: registry.Resolve[...]` ao CompositeLit
// retornado, sem precisar alterar a assinatura do constructor.
func buildRouteFile(domain string, imp importPaths) ([]byte, error) {
	routeType := domain + "Route"

	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-route-create")

	imports := ImportGroups(padder,
		[]string{imp.join(registryImportSubpath)},
		[]string{echoImportPath},
	)

	structType := &ast.StructType{Fields: FieldList()}
	structDecl := TypeDecl(routeType, structType)

	ctorParams := FieldList(
		Field("reg", StarOf(Sel("registry", "Registry"))),
	)
	ctorReturn := &ast.UnaryExpr{Op: token.AND, X: CompositeLit(Ident(routeType))}
	ctorDecl := FuncDecl(
		nil,
		"New"+routeType,
		ctorParams,
		FieldList(AnonField(StarOf(Ident(routeType)))),
		[]ast.Stmt{ReturnStmt(ctorReturn)},
	)

	declarePrivate := buildDeclareMethod(routeType, "DeclarePrivateRoutes")
	declarePublic := buildDeclareMethod(routeType, "DeclarePublicRoutes")

	packagePos := padder.Take()
	padder.Gap(1)

	stampGenDecl(padder, structDecl)
	padder.Gap(1)
	stampFuncDecl(padder, ctorDecl)
	padder.Gap(1)
	stampFuncDecl(padder, declarePrivate)
	padder.Gap(1)
	stampFuncDecl(padder, declarePublic)

	file := &ast.File{
		Package: packagePos,
		Name:    Ident("routes"),
		Decls:   []ast.Decl{imports, structDecl, ctorDecl, declarePrivate, declarePublic},
	}

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

// buildDeclareMethod constrói:
//
//	func (r *<Pascal>Route) <MethodName>(g *echo.Group, prefix string) {
//	}
func buildDeclareMethod(routeType, methodName string) *ast.FuncDecl {
	return FuncDecl(
		PointerReceiver("r", routeType),
		methodName,
		FieldList(
			Field("g", StarOf(Sel("echo", "Group"))),
			Field("prefix", Ident("string")),
		),
		nil,
		[]ast.Stmt{},
	)
}

// stampGenDecl reserva Pos pra TokPos e (se TYPE com struct) Opening/Closing
// das Fields, garantindo layout multi-line consistente.
func stampGenDecl(p *LinePadder, gd *ast.GenDecl) {
	gd.TokPos = p.Take()
	if gd.Tok != token.TYPE {
		return
	}
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok || st.Fields == nil {
			continue
		}
		// Sempre força multi-line: o `route add` precisa de uma estrutura
		// previsível pra inserir novos campos com layout consistente.
		st.Fields.Opening = p.Take()
		st.Fields.Closing = p.Take()
	}
}

// stampFuncDecl reserva Pos pra Func, Lbrace e Rbrace, forçando multi-line.
func stampFuncDecl(p *LinePadder, fd *ast.FuncDecl) {
	fd.Type.Func = p.Take()
	if fd.Body != nil {
		fd.Body.Lbrace = p.Take()
		fd.Body.Rbrace = p.Take()
	}
}
