package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
)

// RouteRemoveOptions parametriza RouteRemove.
type RouteRemoveOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o domain em PascalCase. Identifica o arquivo de rotas alvo
	// (`cmd/http/routes/<snake>.go`) e a struct `<Pascal>Route`.
	Domain string
	// Service é o use case em PascalCase. Junto com Domain forma o par único
	// que identifica o handler 1:1 a ser removido — o campo
	// `<lowerCamel>Handler` dentro da Route é único por construção de
	// `route add`.
	Service string
	// Force permite remoção parcial: se algum dos 4 pontos AST (campo da
	// struct, KV do ctor, ExprStmt em Declare*, import) estiver ausente,
	// remove só o que existir em vez de falhar com `--force` desligado o
	// comportamento é atômico (todos ou nenhum).
	// `assertCtorSignature` continua fatal mesmo com Force — sem o shape
	// canônico não conseguimos localizar o CompositeLit com segurança.
	Force bool
}

// RouteRemove altera `cmd/http/routes/<snake_domain>.go` desfazendo o
// `route add` para o par (Domain, Service). Remove os 4 pontos:
//  1. Campo `<lowerCamel>Handler` na struct `<Pascal>Route`.
//  2. KV `<lowerCamel>Handler: registry.Resolve[...](reg, ...)` no
//     CompositeLit retornado pelo ctor.
//  3. Stmt `g.<METHOD>(... r.<lowerCamel>Handler.Handle)` em
//     `DeclarePrivateRoutes` (preferência) ou `DeclarePublicRoutes`.
//  4. Import do pacote do handler — só se mais nenhum SelectorExpr no
//     arquivo referenciar o pkg ident.
//
// Sem `--force`: pré-check atômico dos 4 pontos antes de mutar. Se algum
// não for achado, falha com mensagem específica e disco intocado.
//
// Com `--force`: pula a checagem atômica e aplica onde os ponteiros forem
// não-nil. Cobre retry após falha intermediária ou estado parcial gerado
// por hand-edit. `assertCtorSignature` continua fatal — sem ele não dá
// pra localizar o CompositeLit com segurança.
//
// Retorna o caminho relativo do arquivo editado.
func RouteRemove(opts RouteRemoveOptions) (string, error) {
	plan, err := planRemove(opts)
	if err != nil {
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

	targets, err := locateRemoveTargets(file, parts, plan, opts.Force)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}

	applyRemovePatches(fset, file, parts, targets, plan)

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// removePlan agrupa nomes derivados e caminhos pra evitar repetição.
type removePlan struct {
	root         string
	relFile      string
	absFile      string
	snakeDomain  string
	snakeService string
	routeType    string
	fieldName    string
	importPath   string
}

func planRemove(opts RouteRemoveOptions) (removePlan, error) {
	var plan removePlan
	if !IsValidExportedIdent(opts.Domain) {
		return plan, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	if !IsValidExportedIdent(opts.Service) {
		return plan, fmt.Errorf("nome de service inválido (esperado PascalCase exportável): %q", opts.Service)
	}

	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	plan.snakeDomain = ToSnake(opts.Domain)
	plan.snakeService = ToSnake(opts.Service)
	plan.routeType = opts.Domain + "Route"
	plan.fieldName = ToLowerCamel(opts.Service) + "Handler"
	plan.relFile = filepath.Join(routesBasePath, plan.snakeDomain+".go")
	plan.absFile = filepath.Join(plan.root, plan.relFile)
	imp, err := newImportPaths(plan.root)
	if err != nil {
		return plan, err
	}
	plan.importPath = imp.join(handlersImportSubpath + "/" + plan.snakeDomain + "/" + plan.snakeService)
	return plan, nil
}

// removeTargets agrupa os nós AST a remover.
type removeTargets struct {
	// field é o campo da struct `<lowerCamel>Handler ...`. nil se ausente.
	field *ast.Field
	// kv é a entrada do CompositeLit do ctor. nil se ausente.
	kv *ast.KeyValueExpr
	// declareTarget é a função Declare*Routes onde mora o stmt — Private
	// preferido, Public como fallback. nil se ausente.
	declareTarget *ast.FuncDecl
	// handlerStmt é o ExprStmt `g.<METHOD>(... r.<fieldName>.Handle)`. nil
	// se ausente.
	handlerStmt ast.Stmt
	// importSpec é o import do handler. nil se ausente.
	importSpec *ast.ImportSpec
}

// locateRemoveTargets resolve os 4 ponteiros. Sem `--force`, falha se
// qualquer um faltar; com `--force`, retorna o que achou e segue.
func locateRemoveTargets(file *ast.File, parts routeParts, plan removePlan, force bool) (removeTargets, error) {
	var t removeTargets

	t.field = findStructField(parts.structType, plan.fieldName)
	t.kv = findCtorKV(parts.ctor, plan.fieldName)
	if stmt := findHandlerStmt(parts.declarePrivate, plan.fieldName); stmt != nil {
		t.handlerStmt = stmt
		t.declareTarget = parts.declarePrivate
	} else if stmt := findHandlerStmt(parts.declarePublic, plan.fieldName); stmt != nil {
		t.handlerStmt = stmt
		t.declareTarget = parts.declarePublic
	}
	t.importSpec = findImportSpec(file, plan.importPath)

	if force {
		return t, nil
	}

	if t.field == nil {
		return t, fmt.Errorf("campo %s.%s ausente", plan.routeType, plan.fieldName)
	}
	if t.kv == nil {
		return t, fmt.Errorf("atribuição %s no construtor New%s ausente", plan.fieldName, plan.routeType)
	}
	if t.handlerStmt == nil {
		return t, fmt.Errorf("chamada r.%s.Handle ausente em DeclarePrivateRoutes/DeclarePublicRoutes", plan.fieldName)
	}
	if t.importSpec == nil {
		return t, fmt.Errorf("import %q ausente", plan.importPath)
	}
	return t, nil
}

// findStructField devolve o *ast.Field cujo único Name bate em fieldName.
// Retorna nil se não houver. Não muta a struct.
func findStructField(st *ast.StructType, fieldName string) *ast.Field {
	if st == nil || st.Fields == nil {
		return nil
	}
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == fieldName {
				return f
			}
		}
	}
	return nil
}

// findCtorKV procura no CompositeLit retornado pelo ctor a entrada
// `<fieldName>: registry.Resolve[...](reg, ...)`. Retorna o
// *ast.KeyValueExpr ou nil. Não muta nada.
func findCtorKV(ctor *ast.FuncDecl, fieldName string) *ast.KeyValueExpr {
	if ctor == nil || ctor.Body == nil {
		return nil
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
		for _, elt := range cl.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			id, ok := kv.Key.(*ast.Ident)
			if !ok || id.Name != fieldName {
				continue
			}
			return kv
		}
	}
	return nil
}

// findHandlerStmt localiza no corpo da função o ExprStmt cuja CallExpr
// contém um arg `r.<fieldName>.Handle` (qualquer um dos args; `route add`
// emite como segundo arg da chamada `g.<METHOD>(...)`).
func findHandlerStmt(fd *ast.FuncDecl, fieldName string) ast.Stmt {
	if fd == nil || fd.Body == nil {
		return nil
	}
	for _, stmt := range fd.Body.List {
		es, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		if exprStmtHasHandlerRef(es, fieldName) {
			return es
		}
	}
	return nil
}

// exprStmtHasHandlerRef devolve true se algum nó dentro do ExprStmt for
// `r.<fieldName>.Handle`.
func exprStmtHasHandlerRef(es *ast.ExprStmt, fieldName string) bool {
	found := false
	ast.Inspect(es, func(n ast.Node) bool {
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

// applyRemovePatches executa as remoções nos 4 pontos. Cada uma só roda
// se o ponteiro alvo for não-nil (relevante quando `--force`).
//
// Ordem importa apenas para a checagem do import: removemos o campo + KV
// + stmt primeiro pra que o `ast.Inspect` final, que decide se o import
// ainda é referenciado, veja o arquivo já sem essas referências.
func applyRemovePatches(fset *token.FileSet, file *ast.File, parts routeParts, t removeTargets, plan removePlan) {
	if t.field != nil {
		removeStructField(parts.structType, t.field)
	}
	if t.kv != nil {
		removeCtorKV(fset, parts.ctor, t.kv)
	}
	if t.handlerStmt != nil && t.declareTarget != nil {
		t.declareTarget.Body.List = removeStmt(t.declareTarget.Body.List, t.handlerStmt)
		collapseTrailingBlank(fset, t.declareTarget.Body)
	}
	if t.importSpec != nil && !pkgIdentReferenced(file, plan.snakeService) {
		aliasName := ""
		if t.importSpec.Name != nil {
			aliasName = t.importSpec.Name.Name
		}
		astutil.DeleteNamedImport(fset, file, aliasName, plan.importPath)
	}
}

// removeStructField remove o *ast.Field de st.Fields.List por igualdade
// de ponteiro.
func removeStructField(st *ast.StructType, target *ast.Field) {
	if st == nil || st.Fields == nil {
		return
	}
	out := make([]*ast.Field, 0, len(st.Fields.List))
	for _, f := range st.Fields.List {
		if f == target {
			continue
		}
		out = append(out, f)
	}
	st.Fields.List = out
}

// removeCtorKV remove o KV do CompositeLit retornado pelo ctor. Se sobrar
// ≥1 KV, restampar via stampCompositeLitDeep + reposicionar Rbrace do ctor
// pra evitar linha em branco residual. Se zerar, deixar o printer/gofmt
// colapsar pra `&<Pascal>Route{}`.
func removeCtorKV(fset *token.FileSet, ctor *ast.FuncDecl, target *ast.KeyValueExpr) {
	if ctor == nil || ctor.Body == nil {
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
		out := make([]ast.Expr, 0, len(cl.Elts))
		for _, elt := range cl.Elts {
			if elt == target {
				continue
			}
			out = append(out, elt)
		}
		cl.Elts = out
		if len(cl.Elts) == 0 {
			// gofmt colapsa pra `&Foo{}` numa única linha; não restampar.
			return
		}
		padder := NewLinePadder(fset, "scaffold-route-remove-ctorlit")
		stampCompositeLitDeep(padder, cl)
		ctor.Body.Rbrace = padder.Take()
		return
	}
}

// pkgIdentReferenced devolve true se algum SelectorExpr `<pkgIdent>.X` no
// arquivo aponta para o pkgIdent (pacote bare = nome importado).
//
// Usado pra decidir se o import do handler ainda é necessário. Cobre
// `<snake_service>.RegistryKey` e `<snake_service>.<Pascal>Handler` que
// outras rotas do mesmo arquivo possam ainda referenciar.
func pkgIdentReferenced(file *ast.File, pkgIdent string) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if id.Name == pkgIdent {
			found = true
			return false
		}
		return true
	})
	return found
}
