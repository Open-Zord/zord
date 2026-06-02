// Package scaffold (área queue / driver omniq) — builders específicos do
// driver `omniq` (github.com/not-empty/omniq-go). Cada outro driver mora
// num arquivo `queue_<driver>.go` separado e se registra em queueDrivers
// (queue_create.go). Os builders agnósticos (repositories/services) +
// helpers genéricos vivem em queue_common.go.
package scaffold

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

// omniqGoImportPath é o módulo Go consumido pelos arquivos gerados deste
// driver. Centralizado pra trocar versão/fork em um único lugar.
const omniqGoImportPath = "github.com/not-empty/omniq-go"

// buildOmniqQueueFiles materializa todos os arquivos do entrypoint quando o
// driver é `omniq`. Ordem é canônica e estável (bootstrap setup→worker → cmd
// main → .env) pra deixar o reporting do QueueCreate e o diff de revisão
// determinísticos.
func buildOmniqQueueFiles(name string, imp importPaths) ([]queueFile, error) {
	bootAlias := stripUnderscores(name) + "boot"
	moduleBootstrap := imp.join(bootstrapBasePath + "/" + name)

	specs := []struct {
		rel string
		fn  func() ([]byte, error)
	}{
		{filepath.Join(bootstrapBasePath, name, "setup.go"), func() ([]byte, error) { return buildOmniqSetupFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "configs.go"), func() ([]byte, error) { return buildOmniqConfigsFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "pkg.go"), func() ([]byte, error) { return buildOmniqPkgFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "repositories.go"), func() ([]byte, error) { return buildQueueRepositoriesFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "services.go"), func() ([]byte, error) { return buildQueueServicesFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "worker.go"), func() ([]byte, error) { return buildOmniqWorkerFile(name, imp) }},
		{filepath.Join(cmdBasePath, name, "main.go"), func() ([]byte, error) { return buildOmniqMainFile(name, bootAlias, moduleBootstrap) }},
		{".env." + name, func() ([]byte, error) { return buildOmniqDotEnvFile(name), nil }},
	}

	out := make([]queueFile, 0, len(specs))
	for _, s := range specs {
		src, err := s.fn()
		if err != nil {
			return nil, fmt.Errorf("gerar %s: %w", s.rel, err)
		}
		out = append(out, queueFile{relPath: s.rel, src: src})
	}
	return out, nil
}

// buildOmniqSetupFile monta `bootstrap/<name>/setup.go` (package doc, import
// block registry+omniq, const omniqClientRegistryKey, func Setup, func Run).
func buildOmniqSetupFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-omniq-setup")

	imports := ImportGroups(padder,
		[]string{imp.join(registryImportSubpath)},
		[]string{omniqGoImportPath},
	)

	constDecl := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{Ident("omniqClientRegistryKey")},
				Values: []ast.Expr{StrLit("omniq.client")},
			},
		},
	}
	constDecl.Doc = singleComment(
		"// omniqClientRegistryKey é a chave do *omniq.Client no registry. Privada ao\n" +
			"// pacote do bootstrap porque só este entrypoint consome o client.")

	// Setup() (reg *registry.Registry, queueName string)
	setupResults := FieldList(
		Field("reg", StarOf(Sel("registry", "Registry"))),
		Field("queueName", Ident("string")),
	)
	confQueueAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("conf"), Ident("queueName")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Ident("loadConfigs")}},
	}
	regNewAssign := Assign(Ident("reg"), &ast.CallExpr{Fun: Sel("registry", "NewRegistry")})
	regPkgCall := callStmt("registerPkg", Ident("reg"), Ident("conf"))
	regRepoCall := callStmt("registerRepositories", Ident("reg"))
	regSvcCall := callStmt("registerServices", Ident("reg"))
	setupReturn := ReturnStmt(Ident("reg"), Ident("queueName"))

	setupDecl := FuncDecl(nil, "Setup", nil, setupResults, []ast.Stmt{
		confQueueAssign, regNewAssign, regPkgCall, regRepoCall, regSvcCall, setupReturn,
	})
	setupDecl.Doc = singleComment(
		"// Setup loads configs and registers primitives, repositories and services\n" +
			"// into the registry in topological order. Returns (registry, queueName).\n" +
			"// Does NOT start the consume loop — use Run() for that. Setup() é separado\n" +
			"// pra que testes e scripts ad-hoc consigam inspecionar o grafo sem assinar\n" +
			"// uma fila.")

	// Run() error
	regQueueDefine := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("reg"), Ident("queueName")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Ident("Setup")}},
	}
	clientResolveCall := &ast.CallExpr{
		Fun:  IndexExpr(Sel("registry", "Resolve"), StarOf(Sel("omniq", "Client"))),
		Args: []ast.Expr{Ident("reg"), Ident("omniqClientRegistryKey")},
	}
	clientDefine := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("client")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{clientResolveCall},
	}
	consumeOpts := &ast.CompositeLit{
		Type: Sel("omniq", "ConsumeOpts"),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: Ident("Queue"), Value: Ident("queueName")},
			&ast.KeyValueExpr{Key: Ident("Handler"), Value: &ast.CallExpr{Fun: Ident("NewHandler"), Args: []ast.Expr{Ident("reg")}}},
			&ast.KeyValueExpr{Key: Ident("Verbose"), Value: Ident("true")},
			&ast.KeyValueExpr{Key: Ident("Drain"), Value: Ident("true")},
		},
	}
	runReturn := ReturnStmt(&ast.CallExpr{
		Fun:  Sel("client", "Consume"),
		Args: []ast.Expr{consumeOpts},
	})
	runDecl := FuncDecl(nil, "Run", nil,
		FieldList(AnonField(Ident("error"))),
		[]ast.Stmt{regQueueDefine, clientDefine, runReturn},
	)
	runDecl.Doc = singleComment(
		"// Run is the long-running entrypoint: wires the graph via Setup() and starts\n" +
			"// the omniq consume loop. Blocks until omniq.Consume returns. One process ==\n" +
			"// one queue == one worker — crash semantics são intencionais, o orquestrador\n" +
			"// (k8s/systemd) reinicia em falha.")

	packageDoc := singleComment(fmt.Sprintf(
		"// Package %[1]s wires the %[1]s queue worker entrypoint (omniq driver) and\n"+
			"// exposes Run() to drive the consume loop. Layout mirrors bootstrap/http/:\n"+
			"// configs.go loads envs, pkg.go registers primitives (logger, db, omniq\n"+
			"// client), repositories.go and services.go are entrypoint-agnostic wire-up,\n"+
			"// worker.go bridges omniq jobs to services via NewHandler(reg).\n"+
			"//\n"+
			"//zord:entrypoint queue_worker", name))

	packageDoc.List[0].Slash = padder.Take()
	packagePos := padder.Take()
	padder.Gap(1)

	stampDeclWithDoc(padder, constDecl)
	padder.Gap(1)

	stampDocPositions(padder, setupDecl.Doc)
	setupDecl.Type.Func = padder.Take()
	setupDecl.Body.Lbrace = padder.Take()
	stampAssignStmt(padder, confQueueAssign)
	stampAssignStmt(padder, regNewAssign)
	stampCallStmt(padder, regPkgCall)
	stampCallStmt(padder, regRepoCall)
	stampCallStmt(padder, regSvcCall)
	setupReturn.Return = padder.Take()
	setupDecl.Body.Rbrace = padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, runDecl.Doc)
	runDecl.Type.Func = padder.Take()
	runDecl.Body.Lbrace = padder.Take()
	stampAssignStmt(padder, regQueueDefine)
	stampAssignStmt(padder, clientDefine)
	runReturn.Return = padder.Take()
	StampCompositeLit(padder, consumeOpts)
	runDecl.Body.Rbrace = padder.Take()

	decls := []ast.Decl{imports, constDecl, setupDecl, runDecl}
	file := &ast.File{
		Doc:     packageDoc,
		Package: packagePos,
		Name:    Ident(name),
		Decls:   decls,
	}
	file.Comments = collectDocs(packageDoc, decls)
	return finalizeASTSource(fset, file)
}

// buildOmniqConfigsFile monta `bootstrap/<name>/configs.go` com const
// envPrefix scoped por entrypoint e loadConfigs() usando LoadEnvsForEntrypoint.
func buildOmniqConfigsFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-omniq-configs")

	imports := ImportGroups(padder, []string{imp.join("pkg/config")})

	envPrefixValue := "OMNIQ_" + strings.ToUpper(name)
	envPrefixConst := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{Ident("envPrefix")},
				Values: []ast.Expr{StrLit(envPrefixValue)},
			},
		},
	}
	envPrefixConst.Doc = singleComment(
		"// envPrefix é o prefixo de todos os envs específicos deste worker (ex.:\n" +
			"// OMNIQ_<NAME>_HOST, OMNIQ_<NAME>_QUEUE). Permite vários workers no mesmo\n" +
			"// .env apontando pra Redis distintos sem conflito de nomes; também é\n" +
			"// reusado pelo pkg.go (mesmo pacote) ao construir o omniq.Client.")

	confAssign := Assign(Ident("conf"), &ast.CallExpr{Fun: Sel("config", "NewConfig")})

	loadEnvsCall := &ast.CallExpr{
		Fun:  Sel("conf", "LoadEnvsForEntrypoint"),
		Args: []ast.Expr{StrLit(name)},
	}
	loadEnvsInit := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("err")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{loadEnvsCall},
	}
	panicStmt := &ast.ExprStmt{X: &ast.CallExpr{Fun: Ident("panic"), Args: []ast.Expr{Ident("err")}}}
	ifLoadEnvs := &ast.IfStmt{
		Init: loadEnvsInit,
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{panicStmt}},
	}

	queueNameAssign := Assign(Ident("queueName"), &ast.CallExpr{
		Fun:  Sel("conf", "ReadConfig"),
		Args: []ast.Expr{prefixedEnvExpr("_QUEUE")},
	})
	returnStmt := ReturnStmt(Ident("conf"), Ident("queueName"))

	results := FieldList(
		Field("conf", StarOf(Sel("config", "Config"))),
		Field("queueName", Ident("string")),
	)
	loadFn := FuncDecl(nil, "loadConfigs", nil, results,
		[]ast.Stmt{confAssign, ifLoadEnvs, queueNameAssign, returnStmt},
	)
	loadFn.Doc = singleComment(
		"// loadConfigs reads the envs and returns the config instance plus the queue\n" +
			"// name. queueName sai por valor porque Run() precisa dele pra chamar\n" +
			"// omniq.Consume; o restante dos envs (host/port/db, primitivos) é lido sob\n" +
			"// demanda em pkg.go via *config.Config.\n" +
			"//\n" +
			"// LoadEnvsForEntrypoint segue a priority list do pkg/config: .env (creds\n" +
			"// reais de homolog, gitignored) > .env.<name> (defaults de dev, versionado)\n" +
			"// > system envs (prod). Em prod nenhum dos arquivos é necessário.")

	packagePos := padder.Take()
	padder.Gap(1)

	stampDeclWithDoc(padder, envPrefixConst)
	padder.Gap(1)

	stampDocPositions(padder, loadFn.Doc)
	loadFn.Type.Func = padder.Take()
	loadFn.Body.Lbrace = padder.Take()
	stampAssignStmt(padder, confAssign)
	ifLoadEnvs.If = padder.Take()
	ifLoadEnvs.Body.Lbrace = padder.Take()
	panicStmt.X.(*ast.CallExpr).Fun.(*ast.Ident).NamePos = padder.Take()
	ifLoadEnvs.Body.Rbrace = padder.Take()
	stampAssignStmt(padder, queueNameAssign)
	returnStmt.Return = padder.Take()
	loadFn.Body.Rbrace = padder.Take()

	decls := []ast.Decl{imports, envPrefixConst, loadFn}
	file := &ast.File{
		Package: packagePos,
		Name:    Ident(name),
		Decls:   decls,
	}
	file.Comments = collectDocs(nil, decls)
	return finalizeASTSource(fset, file)
}

// buildOmniqPkgFile monta `bootstrap/<name>/pkg.go`: registerPkg com
// logger/db/validator/idCreator/omniq client + reg.Provide pra cada um.
func buildOmniqPkgFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-omniq-pkg")

	imports := ImportGroups(padder,
		[]string{
			imp.join("pkg/config"),
			imp.join("pkg/database"),
			imp.join("pkg/idCreator"),
			imp.join("pkg/logger"),
			imp.join(registryImportSubpath),
			imp.join("pkg/validator"),
		},
		[]string{omniqGoImportPath},
	)

	// l := logger.NewLogger(...)
	loggerCall := &ast.CallExpr{
		Fun: Sel("logger", "NewLogger"),
		Args: []ast.Expr{
			readConfigCall("ENVIRONMENT"),
			readConfigCall("APP"),
			readConfigCall("VERSION"),
		},
	}
	lAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("l")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{loggerCall},
	}
	lBoot := callStmt2(Sel("l", "Boot"))

	// db := database.NewMysql(...)
	dbCall := &ast.CallExpr{
		Fun: Sel("database", "NewMysql"),
		Args: []ast.Expr{
			Ident("l"),
			readConfigCall("DB_USER"),
			readConfigCall("DB_PASS"),
			readConfigCall("DB_URL"),
			readConfigCall("DB_PORT"),
			readConfigCall("DB_DATABASE"),
		},
	}
	dbAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("db")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{dbCall},
	}
	dbConnect := callStmt2(Sel("db", "Connect"))

	valAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("val")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Sel("validator", "NewValidator")}},
	}
	valBoot := callStmt2(Sel("val", "Boot"))

	idcAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("idC")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Sel("idCreator", "NewIdCreator")}},
	}

	// client, err := omniq.NewClient(omniq.ClientOpts{Host: ..., Port: ..., DB: ...})
	clientOpts := &ast.CompositeLit{
		Type: Sel("omniq", "ClientOpts"),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: Ident("Host"), Value: prefixedConfigCall("_HOST")},
			&ast.KeyValueExpr{Key: Ident("Port"), Value: prefixedNumberConfigCall("_PORT")},
			&ast.KeyValueExpr{Key: Ident("DB"), Value: prefixedNumberConfigCall("_DB")},
		},
	}
	clientCall := &ast.CallExpr{
		Fun:  Sel("omniq", "NewClient"),
		Args: []ast.Expr{clientOpts},
	}
	clientAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("client"), Ident("err")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{clientCall},
	}

	panicStmt := &ast.ExprStmt{X: &ast.CallExpr{Fun: Ident("panic"), Args: []ast.Expr{Ident("err")}}}
	ifErr := &ast.IfStmt{
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{panicStmt}},
	}

	provideLogger := provideStmt(Sel("logger", "RegistryKey"), Ident("l"))
	provideValidator := provideStmt(Sel("validator", "RegistryKey"), Ident("val"))
	provideConfig := provideStmt(Sel("config", "RegistryKey"), Ident("conf"))
	provideIdCreator := provideStmt(Sel("idCreator", "RegistryKey"), Ident("idC"))
	provideDb := provideStmt(Sel("database", "RegistryKey"), Sel("db", "Db"))
	provideOmniq := provideStmt(Ident("omniqClientRegistryKey"), Ident("client"))

	body := []ast.Stmt{
		lAssign, lBoot,
		dbAssign, dbConnect,
		valAssign, valBoot,
		idcAssign,
		clientAssign, ifErr,
		provideLogger, provideValidator, provideConfig,
		provideIdCreator, provideDb, provideOmniq,
	}

	params := FieldList(
		Field("reg", StarOf(Sel("registry", "Registry"))),
		Field("conf", StarOf(Sel("config", "Config"))),
	)
	registerPkg := FuncDecl(nil, "registerPkg", params, nil, body)
	registerPkg.Doc = singleComment(
		"// registerPkg registers the primitive dependencies: logger, validator, config,\n" +
			"// idCreator, db (*sqlx.DB) e o *omniq.Client. Depende apenas dos configs já\n" +
			"// carregados. Falha rápida no boot se algum env obrigatório estiver ausente.")

	packagePos := padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, registerPkg.Doc)
	registerPkg.Type.Func = padder.Take()
	registerPkg.Body.Lbrace = padder.Take()

	stampAssignStmt(padder, lAssign)
	stampMultilineCallArgs(padder, loggerCall)
	stampCallStmt(padder, lBoot)
	padder.Gap(1)
	stampAssignStmt(padder, dbAssign)
	stampMultilineCallArgs(padder, dbCall)
	stampCallStmt(padder, dbConnect)
	padder.Gap(1)
	stampAssignStmt(padder, valAssign)
	stampCallStmt(padder, valBoot)
	padder.Gap(1)
	stampAssignStmt(padder, idcAssign)
	padder.Gap(1)
	stampAssignStmt(padder, clientAssign)
	StampCompositeLit(padder, clientOpts)
	ifErr.If = padder.Take()
	ifErr.Body.Lbrace = padder.Take()
	panicStmt.X.(*ast.CallExpr).Fun.(*ast.Ident).NamePos = padder.Take()
	ifErr.Body.Rbrace = padder.Take()
	padder.Gap(1)
	stampCallStmt(padder, provideLogger)
	stampCallStmt(padder, provideValidator)
	stampCallStmt(padder, provideConfig)
	stampCallStmt(padder, provideIdCreator)
	stampCallStmt(padder, provideDb)
	stampCallStmt(padder, provideOmniq)
	registerPkg.Body.Rbrace = padder.Take()

	decls := []ast.Decl{imports, registerPkg}
	file := &ast.File{
		Package: packagePos,
		Name:    Ident(name),
		Decls:   decls,
	}
	file.Comments = collectDocs(nil, decls)
	return finalizeASTSource(fset, file)
}

// buildOmniqWorkerFile monta `bootstrap/<name>/worker.go` com
// `func NewHandler(reg *registry.Registry) func(ctx omniq.JobCtx)` + stub.
func buildOmniqWorkerFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-omniq-worker")

	imports := ImportGroups(padder,
		[]string{imp.join(registryImportSubpath)},
		[]string{omniqGoImportPath},
	)

	discardAssign := Assign(Ident("_"), Ident("reg"))

	handlerFnType := &ast.FuncType{
		Params: FieldList(Field("ctx", Sel("omniq", "JobCtx"))),
	}
	handlerLit := &ast.FuncLit{
		Type: handlerFnType,
		Body: &ast.BlockStmt{},
	}
	handlerComments := &ast.CommentGroup{List: []*ast.Comment{
		{Text: "// 1. decodificar payload com ctx.DecodePayload(&p)"},
		{Text: "// 2. chamar service(s) resolvidos via registry.Resolve no escopo externo"},
		{Text: "// 3. panic em erro irrecuperável → omniq marca o job como FAILED"},
	}}

	returnClosure := ReturnStmt(handlerLit)

	params := FieldList(Field("reg", StarOf(Sel("registry", "Registry"))))
	// omniq-go não re-exporta o type alias ConsumeHandler do pacote internal,
	// então declara o function type inline. ConsumeOpts.Handler é assignável
	// a `func(omniq.JobCtx)` por equivalência estrutural.
	handlerResultType := &ast.FuncType{
		Params: FieldList(Field("ctx", Sel("omniq", "JobCtx"))),
	}
	results := FieldList(AnonField(handlerResultType))
	fn := FuncDecl(nil, "NewHandler", params, results, []ast.Stmt{discardAssign, returnClosure})
	fn.Doc = singleComment(
		"// NewHandler constrói o ConsumeHandler do worker. Resolva services/repos do\n" +
			"// registry aqui (closure) e use-os dentro do handler retornado. Não é\n" +
			"// registrado no registry porque há apenas um worker por entrypoint — Run()\n" +
			"// constrói direto.\n" +
			"//\n" +
			"// Contrato omniq: handler retornando normalmente == job ACKed; panic ==\n" +
			"// job marcado como FAILED e candidato a retry (até MaxAttempts). Use\n" +
			"// ctx.DecodePayload(&p) pra extrair o payload tipado.")

	packagePos := padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, fn.Doc)
	fn.Type.Func = padder.Take()
	fn.Body.Lbrace = padder.Take()
	stampAssignStmt(padder, discardAssign)
	returnClosure.Return = padder.Take()
	handlerLit.Type.Func = padder.Take()
	handlerLit.Body.Lbrace = padder.Take()
	for _, c := range handlerComments.List {
		c.Slash = padder.Take()
	}
	handlerLit.Body.Rbrace = padder.Take()
	fn.Body.Rbrace = padder.Take()

	decls := []ast.Decl{imports, fn}
	file := &ast.File{
		Package: packagePos,
		Name:    Ident(name),
		Decls:   decls,
	}
	file.Comments = append(collectDocs(nil, decls), handlerComments)
	return finalizeASTSource(fset, file)
}

// buildOmniqMainFile monta `cmd/<name>/main.go`: import log + alias do
// bootstrap, func main com if-err `log.Fatalf` em caso de erro do Run().
func buildOmniqMainFile(name, bootAlias, bootImportPath string) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-omniq-main")

	// Import block manual: "log" (stdlib) + bootAlias "bootImportPath" (aliased).
	logSpec := ImportSpec("log")
	bootSpec := ImportSpec(bootImportPath)
	bootSpec.Name = Ident(bootAlias)
	lparen := padder.Take()
	logSpec.Path.ValuePos = padder.Take()
	padder.Gap(1)
	bootSpec.Path.ValuePos = padder.Take()
	bootSpec.Name.NamePos = bootSpec.Path.ValuePos
	rparen := padder.Take()
	imports := &ast.GenDecl{
		Tok:    token.IMPORT,
		Lparen: lparen,
		Rparen: rparen,
		Specs:  []ast.Spec{logSpec, bootSpec},
	}

	// if err := <alias>.Run(); err != nil { log.Fatalf("queue worker: %v", err) }
	runCall := &ast.CallExpr{Fun: Sel(bootAlias, "Run")}
	ifInit := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("err")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{runCall},
	}
	fatalCall := &ast.ExprStmt{X: &ast.CallExpr{
		Fun:  Sel("log", "Fatalf"),
		Args: []ast.Expr{StrLit("queue worker: %v"), Ident("err")},
	}}
	ifStmt := &ast.IfStmt{
		Init: ifInit,
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{fatalCall}},
	}

	mainDecl := FuncDecl(nil, "main", nil, nil, []ast.Stmt{ifStmt})

	packageDoc := singleComment(fmt.Sprintf(
		"// Command %s é o entrypoint de worker (esqueleto gerado por scaffold queue create).", name))

	packageDoc.List[0].Slash = padder.Take()
	packagePos := padder.Take()
	padder.Gap(1)
	imports.TokPos = padder.Take()
	padder.Gap(1)

	mainDecl.Type.Func = padder.Take()
	mainDecl.Body.Lbrace = padder.Take()
	ifStmt.If = padder.Take()
	ifStmt.Body.Lbrace = padder.Take()
	stampCallStmt(padder, fatalCall)
	ifStmt.Body.Rbrace = padder.Take()
	mainDecl.Body.Rbrace = padder.Take()

	decls := []ast.Decl{imports, mainDecl}
	file := &ast.File{
		Doc:     packageDoc,
		Package: packagePos,
		Name:    Ident("main"),
		Decls:   decls,
	}
	file.Comments = collectDocs(packageDoc, decls)
	return finalizeASTSource(fset, file)
}

// buildOmniqDotEnvFile monta `.env.<name>` na raiz do repo — defaults
// VERSIONADOS pra dev local. Não-Go: texto puro com banner avisando que
// segredos reais não devem entrar aqui (vai pro git). Para creds de
// homolog/staging, o dev cria `.env` na raiz (gitignored, prioridade 1).
func buildOmniqDotEnvFile(name string) []byte {
	upper := strings.ToUpper(name)
	body := fmt.Sprintf(`# ============================================================================
# .env.%[1]s — config DE DEV LOCAL pra o entrypoint %[1]s
# ============================================================================
# Este arquivo é COMMITADO. Serve de template/default pra rodar o entrypoint
# logo após clonar o repo (apontando pra serviços locais: localhost, etc.).
#
# >>> NÃO COLOQUE CREDENCIAIS REAIS AQUI. <<<
# Nada de senha de banco de prod, tokens de API, keys de cloud, etc. Tudo
# que entra aqui vai pro git e fica visível pra todo mundo com acesso ao repo.
#
# Pra valores sensíveis (homolog/staging), crie um .env na raiz do repo —
# ele tem prioridade 1 e está no .gitignore.
#
# Em produção, o binário lê só envs do sistema; nenhum .env é necessário.
# ============================================================================

OMNIQ_%[2]s_HOST=localhost
OMNIQ_%[2]s_PORT=6379
OMNIQ_%[2]s_DB=0
OMNIQ_%[2]s_QUEUE=%[1]s
`, name, upper)
	return []byte(body)
}
