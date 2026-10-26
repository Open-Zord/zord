package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
)

const (
	declarableRelPath = "cmd/http/routes/declarable.go"
	getRoutesFunc     = "GetRoutes"
)

// RouteRegisterOptions parametriza RouteRegister.
type RouteRegisterOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Namespace", "UsageRecord").
	// Determina a chave do map (`<snake>`) e o nome do constructor da Route
	// (`New<Pascal>Route`).
	Domain string
}

// RouteRegister patcha `cmd/http/routes/declarable.go` adicionando a entrada
//
//	"<snake_domain>": New<Pascal>Route(reg)
//
// ao fim do map literal retornado por `GetRoutes`. Retorna o caminho relativo
// do arquivo editado.
//
// A Route precisa ter sido gerada pelo shape canônico da NAVE-74 — o
// constructor `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route` é
// validado pelo mesmo helper usado pelo `route add` (`assertCtorSignature`).
// Routes hand-editadas com parâmetros extras falham com mensagem clara
// apontando para registro manual.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain é PascalCase exportável.
//   - O arquivo da Route (`cmd/http/routes/<snake>.go`) existe e contém a
//     struct `<Pascal>Route` + o constructor `New<Pascal>Route(reg *registry.Registry)`.
//   - `cmd/http/routes/declarable.go` existe e tem `func GetRoutes(...)`
//     terminando em `return map[string]Declarable{...}`.
//   - A chave `"<snake_domain>"` ainda não está presente no map (idempotente:
//     re-executar para o mesmo domain sempre falha).
//
// Imports e blocos de Resolve em `declarable.go` ficam intocados — a Route
// resolve os próprios handlers internamente desde a NAVE-74.
func RouteRegister(opts RouteRegisterOptions) (string, error) {
	plan, err := planRegister(opts)
	if err != nil {
		return "", err
	}
	if err := assertRouteConforms(plan.root, plan.snakeDomain, plan.routeType); err != nil {
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

	fn, err := findFreeFuncDecl(file, getRoutesFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	mapLit, err := findRoutesMapLit(fn)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	if hasRouteEntry(mapLit, plan.snakeDomain) {
		return "", fmt.Errorf("em %s: entrada %q já presente em GetRoutes", plan.relFile, plan.snakeDomain)
	}

	mapLit.Elts = append(mapLit.Elts, buildRouteEntry(plan.snakeDomain, plan.routeType))
	// Stamping é necessário porque o append entrega o novo KeyValueExpr com
	// Pos zero; sem isso, go/printer cola a entrada nova na linha da última
	// entrada histórica (`"org": NewOrgRoute(orgH), "<snake>": ...`). Após
	// `stampMapLit`, gofmt em writeFile re-alinha colunas das chaves. O
	// Rbrace do body do GetRoutes também é re-stampado — sem isso, sobra
	// uma linha em branco entre o `}` do CompositeLit e o `}` da função
	// porque a Pos original do Rbrace fica muito além do CompositeLit
	// padded (mesmo padrão de route/add.go:437).
	padder := NewLinePadder(fset, "scaffold-route-register")
	stampMapLit(padder, mapLit)
	fn.Body.Rbrace = padder.Take()

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// registerPlan agrupa nomes derivados e caminhos pra evitar repetir a derivação
// nos vários passos de RouteRegister.
type registerPlan struct {
	root        string
	relFile     string
	absFile     string
	snakeDomain string
	routeType   string
}

func planRegister(opts RouteRegisterOptions) (registerPlan, error) {
	var plan registerPlan
	if !IsValidExportedIdent(opts.Domain) {
		return plan, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	plan.snakeDomain = ToSnake(opts.Domain)
	plan.routeType = opts.Domain + "Route"
	plan.relFile = declarableRelPath
	plan.absFile = filepath.Join(plan.root, plan.relFile)
	return plan, nil
}

// assertRouteConforms confirma que `cmd/http/routes/<snake>.go` existe e
// expõe o shape canônico NAVE-74: struct `<Pascal>Route` + constructor
// `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route`. Reusa
// `assertCtorSignature` do `route add` — single source of truth do shape.
func assertRouteConforms(root, snakeDomain, routeType string) error {
	relFile := filepath.Join(routesBasePath, snakeDomain+".go")
	absFile := filepath.Join(root, relFile)
	//nolint:gosec // G304: path derives from validated identifier, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("ler route %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	if !hasStruct(file, routeType) {
		return fmt.Errorf("em %s: struct %s não encontrada", relFile, routeType)
	}
	ctorName := "New" + routeType
	ctor, err := findFreeFuncDecl(file, ctorName)
	if err != nil {
		return fmt.Errorf("em %s: %w", relFile, err)
	}
	if err := assertCtorSignature(ctor, ctorName); err != nil {
		return fmt.Errorf("em %s: %w", relFile, err)
	}
	return nil
}

// findFreeFuncDecl localiza uma função top-level (sem receiver) pelo nome.
// Erro se não existir ou se o corpo for nil.
func findFreeFuncDecl(file *ast.File, funcName string) (*ast.FuncDecl, error) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name == nil {
			continue
		}
		if fd.Name.Name != funcName {
			continue
		}
		if fd.Body == nil {
			return nil, fmt.Errorf("func %s não tem corpo", funcName)
		}
		return fd, nil
	}
	return nil, fmt.Errorf("func %s não encontrada", funcName)
}

// findRoutesMapLit extrai o `*ast.CompositeLit` do `ReturnStmt` final de
// `GetRoutes`. Valida que o tipo retornado é `map[string]Declarable`. Sem
// essa validação, um arquivo `declarable.go` re-escrito poderia mascarar
// um erro silencioso (entrada inserida em retorno errado).
func findRoutesMapLit(fn *ast.FuncDecl) (*ast.CompositeLit, error) {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return nil, fmt.Errorf("func %s sem corpo", fn.Name.Name)
	}
	ret, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, fmt.Errorf("func %s não termina com `return <map literal>`", fn.Name.Name)
	}
	cl, ok := ret.Results[0].(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("func %s: retorno não é composite literal", fn.Name.Name)
	}
	mt, ok := cl.Type.(*ast.MapType)
	if !ok {
		return nil, fmt.Errorf("func %s: retorno não é map literal", fn.Name.Name)
	}
	if id, ok := mt.Key.(*ast.Ident); !ok || id.Name != "string" {
		return nil, fmt.Errorf("func %s: chave do map não é string", fn.Name.Name)
	}
	if id, ok := mt.Value.(*ast.Ident); !ok || id.Name != "Declarable" {
		return nil, fmt.Errorf("func %s: valor do map não é Declarable", fn.Name.Name)
	}
	return cl, nil
}

// hasRouteEntry devolve true se já existe um `*ast.KeyValueExpr` no map
// cuja chave seja a string literal `"<snake_domain>"`. Comparação estrutural
// — não pega falso positivo em comentários ou em outros KVs.
func hasRouteEntry(cl *ast.CompositeLit, snakeDomain string) bool {
	quoted := strconv.Quote(snakeDomain)
	for _, e := range cl.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		lit, ok := kv.Key.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if lit.Value == quoted {
			return true
		}
	}
	return false
}

// buildRouteEntry monta o `KeyValueExpr`:
//
//	"<snake_domain>": New<Pascal>Route(reg)
//
// Pos zero é proposital — `stampMapLit` define a linha de cada KV num passo
// posterior, junto com o re-alinhamento das entradas históricas.
func buildRouteEntry(snakeDomain, routeType string) *ast.KeyValueExpr {
	return &ast.KeyValueExpr{
		Key: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(snakeDomain)},
		Value: &ast.CallExpr{
			Fun:  ast.NewIdent("New" + routeType),
			Args: []ast.Expr{ast.NewIdent("reg")},
		},
	}
}

// stampMapLit força cada KV do map literal a ficar numa linha distinta
// reservando uma linha do LinePadder por entrada. Diferente do
// stampCompositeLitDeep (route/common.go), trata Keys do tipo
// *ast.BasicLit (string literal) e CallExpr com Fun=*ast.Ident — shape
// específico do map retornado por GetRoutes (`"<snake>": NewXRoute(reg)`).
//
// Re-stampa todos os KVs (não só o novo): após o append, o gofmt em
// writeFile precisa de uma linha por KV pra decidir layout corretamente.
// As Pos originais das entradas históricas perdem fidelidade, mas o
// alinhamento de colunas é re-derivado pelo próprio gofmt — não depende
// das Pos.
func stampMapLit(padder *LinePadder, cl *ast.CompositeLit) {
	cl.Lbrace = padder.Take()
	for _, e := range cl.Elts {
		kvLine := padder.Take()
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if lit, ok := kv.Key.(*ast.BasicLit); ok {
			lit.ValuePos = kvLine
		}
		kv.Colon = kvLine
		ce, ok := kv.Value.(*ast.CallExpr)
		if !ok {
			continue
		}
		if id, ok := ce.Fun.(*ast.Ident); ok {
			id.NamePos = kvLine
		}
		ce.Lparen = kvLine
		ce.Rparen = kvLine
		for _, arg := range ce.Args {
			stampExprLine(arg, kvLine)
		}
	}
	cl.Rbrace = padder.Take()
}
