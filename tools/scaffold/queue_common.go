// Package scaffold (área queue / common) — builders e helpers
// driver-agnósticos compartilhados por todos os drivers de fila. Esse
// arquivo é o ponto de extensão pra coisas que NÃO devem viver dentro do
// builder de um driver específico (queue_<driver>.go).
//
// O critério pra mover algo pra cá: o builder/helper é usado por mais de um
// driver E não introduz dep transitiva no SDK do driver (omniq-go, sdk-aws,
// etc.). Tudo que toca a API de um driver fica no arquivo desse driver.
package scaffold

import (
	"go/ast"
	"go/token"
)

// buildQueueRepositoriesFile / buildQueueServicesFile: stubs idênticos em
// shape a bootstrap/http/{repositories,services}.go. Os mesmos arquivos
// servem qualquer driver — repositórios/services são camadas
// transport-agnostic do hexagonal.

func buildQueueRepositoriesFile(name string, imp importPaths) ([]byte, error) {
	return buildEmptyRegisterFile(
		name, imp, "scaffold-queue-repositories",
		"registerRepositories",
		"// registerRepositories registers the persistence repositories. Depende apenas\n"+
			"// de db (*sqlx.DB) já registrado por pkg.go. A scaffold tool apende novos\n"+
			"// repositórios aqui.",
	)
}

func buildQueueServicesFile(name string, imp importPaths) ([]byte, error) {
	return buildEmptyRegisterFile(
		name, imp, "scaffold-queue-services",
		"registerServices",
		"// registerServices builds and registers the application services (use cases)\n"+
			"// into the registry. Services resolvem primitivos e repositórios do registry\n"+
			"// e são construídos eagermente, então uma dep faltando falha rápido no boot.\n"+
			"// A scaffold tool apende novos services aqui.",
	)
}

// buildEmptyRegisterFile monta um arquivo com import single de pkg/registry
// e uma única func `<fnName>(reg *registry.Registry) {}` (corpo vazio,
// idêntico em shape aos stubs de bootstrap/http/services.go e handlers.go).
func buildEmptyRegisterFile(name string, imp importPaths, padderName, fnName, doc string) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, padderName)

	imports := ImportGroups(padder, []string{imp.join(registryImportSubpath)})

	params := FieldList(Field("reg", StarOf(Sel("registry", "Registry"))))
	fn := FuncDecl(nil, fnName, params, nil, nil)
	fn.Doc = singleComment(doc)

	packagePos := padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, fn.Doc)
	fn.Type.Func = padder.Take()
	fn.Body.Lbrace = padder.Take()
	fn.Body.Rbrace = padder.Take()

	decls := []ast.Decl{imports, fn}
	file := &ast.File{
		Package: packagePos,
		Name:    Ident(name),
		Decls:   decls,
	}
	file.Comments = collectDocs(nil, decls)
	return finalizeASTSource(fset, file)
}

// --- helpers AST de queue (envs scoped + statement stamping) ---
//
// Os helpers abaixo vivem aqui porque o estilo de "stamp uma linha por
// statement" foi introduzido pelos templates de queue e ainda não é
// suficientemente genérico pra ir ao astbuild.go. Quando outro scaffold
// precisar (e o shape provar estável), promovo.

// readConfigCall monta `conf.ReadConfig("<key>")`.
func readConfigCall(key string) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  Sel("conf", "ReadConfig"),
		Args: []ast.Expr{StrLit(key)},
	}
}

// prefixedEnvExpr monta a BinaryExpr `envPrefix + "<suffix>"` usada nos
// templates de configs.go e pkg.go pra construir chaves de env scoped por
// entrypoint sem hardcodar o prefixo em cada call site.
func prefixedEnvExpr(suffix string) ast.Expr {
	return &ast.BinaryExpr{
		X:  Ident("envPrefix"),
		Op: token.ADD,
		Y:  StrLit(suffix),
	}
}

// prefixedConfigCall monta `conf.ReadConfig(envPrefix + "<suffix>")` —
// scoped por entrypoint (configs.go declara envPrefix como const local).
func prefixedConfigCall(suffix string) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  Sel("conf", "ReadConfig"),
		Args: []ast.Expr{prefixedEnvExpr(suffix)},
	}
}

// prefixedNumberConfigCall monta `conf.ReadNumberConfig(envPrefix + "<suffix>")`.
func prefixedNumberConfigCall(suffix string) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  Sel("conf", "ReadNumberConfig"),
		Args: []ast.Expr{prefixedEnvExpr(suffix)},
	}
}

// provideStmt monta `reg.Provide(<key>, <value>)` como ExprStmt.
func provideStmt(key, value ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{X: &ast.CallExpr{
		Fun:  Sel("reg", "Provide"),
		Args: []ast.Expr{key, value},
	}}
}

// callStmt monta `<fn>(<args>...)` como ExprStmt onde fn é um identificador
// livre (sem receiver). Pra chamadas com seletor (ex.: `db.Connect()`), usar
// callStmt2.
func callStmt(fn string, args ...ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{X: &ast.CallExpr{Fun: Ident(fn), Args: args}}
}

// callStmt2 monta `<sel>(<args>...)` como ExprStmt onde sel é uma SelectorExpr.
func callStmt2(sel *ast.SelectorExpr, args ...ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{X: &ast.CallExpr{Fun: sel, Args: args}}
}

// stampAssignStmt reserva uma linha pro TokPos do AssignStmt (e a NamePos do
// primeiro Ident LHS), garantindo que o printer não cole o stmt na linha
// anterior.
func stampAssignStmt(p *LinePadder, as *ast.AssignStmt) {
	pos := p.Take()
	as.TokPos = pos
	if len(as.Lhs) > 0 {
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			id.NamePos = pos
		}
	}
}

// stampCallStmt reserva uma linha pra Pos do ExprStmt(CallExpr), via Fun
// (Ident ou SelectorExpr).
func stampCallStmt(p *LinePadder, es *ast.ExprStmt) {
	call, ok := es.X.(*ast.CallExpr)
	if !ok {
		return
	}
	pos := p.Take()
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		fn.NamePos = pos
	case *ast.SelectorExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			id.NamePos = pos
		}
	}
}

// stampMultilineCallArgs força o CallExpr a sair multi-line atribuindo Pos
// distintas a Lparen, ao primeiro nó posicionável de cada arg e ao Rparen.
// Sem isso, gofmt colapsa args numa linha só quando o conteúdo "cabe" (sem
// limite de coluna), o que fica feio em construtores com 4+ params.
func stampMultilineCallArgs(p *LinePadder, call *ast.CallExpr) {
	if len(call.Args) == 0 {
		return
	}
	call.Lparen = p.Take()
	for _, arg := range call.Args {
		stampExprPos(p, arg)
	}
	call.Rparen = p.Take()
}

// stampExprPos atribui Pos ao nó posicionável principal da expressão. Cobre
// os casos usados pelos templates (Ident, BasicLit, CallExpr, SelectorExpr);
// outros tipos consumem uma linha do padder mas não posicionam nada.
func stampExprPos(p *LinePadder, e ast.Expr) {
	pos := p.Take()
	switch x := e.(type) {
	case *ast.Ident:
		x.NamePos = pos
	case *ast.BasicLit:
		x.ValuePos = pos
	case *ast.CallExpr:
		switch fn := x.Fun.(type) {
		case *ast.Ident:
			fn.NamePos = pos
		case *ast.SelectorExpr:
			if id, ok := fn.X.(*ast.Ident); ok {
				id.NamePos = pos
			}
		}
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			id.NamePos = pos
		}
	}
}
