// Package scaffold (área queue) cria o esqueleto de um entrypoint de worker
// de fila. Hoje só o driver `omniq` (github.com/not-empty/omniq-go) é
// suportado, mas o flag `--driver` está pronto pra ganhar outros drivers
// (sqs, rabbitmq, kafka) sem renomear o comando.
//
// Filosofia: 1 entrypoint == 1 fila == 1 worker. O nome da fila Redis vem
// do env `OMNIQ_QUEUE` em runtime — o nome do entrypoint (segmento de
// pacote Go) é só convenção de código. Crash do consume loop derruba o
// processo; orquestrador externo (k8s/systemd) reinicia.
//
// Estrutura gerada espelha `bootstrap/http/` por simetria: 6 arquivos em
// `bootstrap/<name>/` (setup, configs, pkg, repositories, services, worker)
// + `cmd/<name>/main.go`. `repositories.go` e `services.go` saem com corpo
// vazio — quando os scaffolds existentes virarem entrypoint-aware
// (`--entrypoint`), continuam funcionando sem mudança nesses dois arquivos.
//
// Geração via AST puro (mesma regra do resto do scaffold): cada arquivo é
// montado nó-a-nó com os builders de astbuild.go, posicionado via LinePadder
// e formatado pelo printer + gofmt. Dá safety contra refactor das APIs
// referenciadas — renomear `registry.NewRegistry` quebra o gerador em
// compile-time, não no runtime do usuário.
package scaffold

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
)

// QueueDriver enumera os drivers de fila que o scaffold sabe gerar. Por ora
// só `omniq` está implementado; manter como tipo nomeado deixa o switch no
// gerador local e o erro de driver desconhecido vira validação centralizada.
type QueueDriver string

const (
	// QueueDriverOmniq gera o entrypoint contra github.com/not-empty/omniq-go.
	QueueDriverOmniq QueueDriver = "omniq"

	omniqGoImportPath = "github.com/not-empty/omniq-go"
)

// QueueCreateOptions parametriza QueueCreate.
type QueueCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Name é o nome do entrypoint em lowercase (ex.: "worker",
	// "billing_worker"). Vira o pacote Go em bootstrap/<name>/ e o segmento
	// em cmd/<name>/. Não é o nome da fila Redis — esse vem do env
	// OMNIQ_QUEUE em runtime.
	Name string
	// Driver seleciona o backend da fila. Vazio default pra QueueDriverOmniq.
	Driver QueueDriver
}

// QueueCreate gera o esqueleto completo do entrypoint de worker:
//
//   - bootstrap/<name>/setup.go      — Setup() + Run()
//   - bootstrap/<name>/configs.go    — loadConfigs (envs + queue name)
//   - bootstrap/<name>/pkg.go        — registerPkg (logger, db, omniq client, ...)
//   - bootstrap/<name>/repositories.go — registerRepositories (vazio)
//   - bootstrap/<name>/services.go   — registerServices (vazio)
//   - bootstrap/<name>/worker.go     — NewHandler(reg) stub
//   - cmd/<name>/main.go             — chama <name>boot.Run()
//
// Retorna a lista de caminhos relativos editados na ordem acima.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Name é identificador Go válido em lowercase (não keyword).
//   - Driver é um QueueDriver suportado (vazio == omniq default).
//   - Nenhum dos diretórios alvo (cmd/<name>/ ou bootstrap/<name>/) existe.
func QueueCreate(opts QueueCreateOptions) ([]string, error) {
	if !IsValidPackageIdent(opts.Name) {
		return nil, fmt.Errorf("nome de entrypoint inválido (esperado lowercase Go ident, não keyword): %q", opts.Name)
	}
	driver := opts.Driver
	if driver == "" {
		driver = QueueDriverOmniq
	}
	if driver != QueueDriverOmniq {
		return nil, fmt.Errorf("driver de fila não suportado: %q (suportados: %q)", driver, QueueDriverOmniq)
	}

	root := opts.Root
	if root == "" {
		root = "."
	}

	relBootstrapDir := filepath.Join(bootstrapBasePath, opts.Name)
	relCmdDir := filepath.Join(cmdBasePath, opts.Name)
	absBootstrapDir := filepath.Join(root, relBootstrapDir)
	absCmdDir := filepath.Join(root, relCmdDir)

	if err := assertDirAbsent(absBootstrapDir, relBootstrapDir); err != nil {
		return nil, err
	}
	if err := assertDirAbsent(absCmdDir, relCmdDir); err != nil {
		return nil, err
	}

	imp, err := newImportPaths(root)
	if err != nil {
		return nil, err
	}

	files, err := buildQueueOmniqFiles(opts.Name, imp)
	if err != nil {
		return nil, err
	}

	// Mutação: bootstrap inteiro primeiro, cmd depois. Mesma postura do
	// entrypoint_create — falha parcial é reportada (sem rollback), o dev
	// resolve manualmente. Ordem dos arquivos dentro de bootstrap é
	// determinística pra facilitar diff revisão.
	created := make([]string, 0, len(files))
	for _, f := range files {
		absFile := filepath.Join(root, f.relPath)
		absDir := filepath.Dir(absFile)
		if err := writeNewFile(absDir, absFile, f.src); err != nil {
			return nil, err
		}
		created = append(created, f.relPath)
	}
	return created, nil
}

type queueFile struct {
	relPath string
	src     []byte
}

// buildQueueOmniqFiles materializa os 7 arquivos do entrypoint omniq na
// ordem canônica (bootstrap → cmd). Cada builder devolve source Go já
// formatado por finalizeASTSource (format.Node + gofmt). Se algum AST
// estiver mal-construído, falha aqui antes de tocar o disco.
func buildQueueOmniqFiles(name string, imp importPaths) ([]queueFile, error) {
	bootAlias := stripUnderscores(name) + "boot"
	moduleBootstrap := imp.join(bootstrapBasePath + "/" + name)

	specs := []struct {
		rel string
		fn  func() ([]byte, error)
	}{
		{filepath.Join(bootstrapBasePath, name, "setup.go"), func() ([]byte, error) { return buildQueueSetupFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "configs.go"), func() ([]byte, error) { return buildQueueConfigsFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "pkg.go"), func() ([]byte, error) { return buildQueuePkgFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "repositories.go"), func() ([]byte, error) { return buildQueueRepositoriesFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "services.go"), func() ([]byte, error) { return buildQueueServicesFile(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "worker.go"), func() ([]byte, error) { return buildQueueWorkerFile(name, imp) }},
		{filepath.Join(cmdBasePath, name, "main.go"), func() ([]byte, error) { return buildQueueMainFile(name, bootAlias, moduleBootstrap) }},
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

// --- builders ---

// buildQueueSetupFile monta `bootstrap/<name>/setup.go` (package doc, import
// block registry+omniq, const omniqClientRegistryKey, func Setup, func Run).
func buildQueueSetupFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-setup")

	imports := ImportGroups(padder,
		[]string{imp.join(registryImportSubpath)},
		[]string{omniqGoImportPath},
	)

	// const omniqClientRegistryKey = "omniq.client"
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

	// Setup() (reg *registry.Registry, queueName string) { ... }
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

	setupBody := []ast.Stmt{
		confQueueAssign,
		regNewAssign,
		regPkgCall,
		regRepoCall,
		regSvcCall,
		setupReturn,
	}
	setupDecl := FuncDecl(nil, "Setup", nil, setupResults, setupBody)
	setupDecl.Doc = singleComment(
		"// Setup loads configs and registers primitives, repositories and services\n" +
			"// into the registry in topological order. Returns (registry, queueName).\n" +
			"// Does NOT start the consume loop — use Run() for that. Setup() é separado\n" +
			"// pra que testes e scripts ad-hoc consigam inspecionar o grafo sem assinar\n" +
			"// uma fila.")

	// Run() error { ... }
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
			"// worker.go bridges omniq jobs to services via NewHandler(reg).", name))

	// Layout stamping: pkg doc, package, gap, imports, gap, const, gap, Setup, gap, Run.
	packageDoc.List[0].Slash = padder.Take()
	packagePos := padder.Take()
	padder.Gap(1)

	// Const
	stampDeclWithDoc(padder, constDecl)
	padder.Gap(1)

	// Setup body — stamp each stmt pra forçar uma por linha.
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

	// Run body
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

// buildQueueConfigsFile monta `bootstrap/<name>/configs.go`.
func buildQueueConfigsFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-configs")

	imports := ImportGroups(padder, []string{imp.join("pkg/config")})

	// conf = config.NewConfig()
	confAssign := Assign(Ident("conf"), &ast.CallExpr{Fun: Sel("config", "NewConfig")})

	// if err := conf.LoadEnvs(); err != nil { panic(err) }
	loadEnvsCall := &ast.CallExpr{Fun: Sel("conf", "LoadEnvs")}
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

	// queueName = conf.ReadConfig("OMNIQ_QUEUE")
	queueNameAssign := Assign(Ident("queueName"), &ast.CallExpr{
		Fun:  Sel("conf", "ReadConfig"),
		Args: []ast.Expr{StrLit("OMNIQ_QUEUE")},
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
			"// demanda em pkg.go via *config.Config.")

	// Layout stamping
	packagePos := padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, loadFn.Doc)
	loadFn.Type.Func = padder.Take()
	loadFn.Body.Lbrace = padder.Take()
	stampAssignStmt(padder, confAssign)
	// if-stmt: stamp If position + body lbrace/rbrace.
	ifLoadEnvs.If = padder.Take()
	ifLoadEnvs.Body.Lbrace = padder.Take()
	panicStmt.X.(*ast.CallExpr).Fun.(*ast.Ident).NamePos = padder.Take()
	ifLoadEnvs.Body.Rbrace = padder.Take()
	stampAssignStmt(padder, queueNameAssign)
	returnStmt.Return = padder.Take()
	loadFn.Body.Rbrace = padder.Take()

	decls := []ast.Decl{imports, loadFn}
	file := &ast.File{
		Package: packagePos,
		Name:    Ident(name),
		Decls:   decls,
	}
	file.Comments = collectDocs(nil, decls)
	return finalizeASTSource(fset, file)
}

// buildQueuePkgFile monta `bootstrap/<name>/pkg.go`: registerPkg com
// logger/db/validator/idCreator/omniq client + reg.Provide pra cada um.
func buildQueuePkgFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-pkg")

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

	// l := logger.NewLogger(conf.ReadConfig("ENVIRONMENT"), ...)
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

	// db := database.NewMysql(l, conf.ReadConfig(...), ...)
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

	// val := validator.NewValidator(); val.Boot()
	valAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("val")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Sel("validator", "NewValidator")}},
	}
	valBoot := callStmt2(Sel("val", "Boot"))

	// idC := idCreator.NewIdCreator()
	idcAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("idC")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Sel("idCreator", "NewIdCreator")}},
	}

	// client, err := omniq.NewClient(omniq.ClientOpts{Host: ..., Port: ..., DB: ...})
	clientOpts := &ast.CompositeLit{
		Type: Sel("omniq", "ClientOpts"),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: Ident("Host"), Value: readConfigCall("OMNIQ_HOST")},
			&ast.KeyValueExpr{Key: Ident("Port"), Value: readNumberConfigCall("OMNIQ_PORT")},
			&ast.KeyValueExpr{Key: Ident("DB"), Value: readNumberConfigCall("OMNIQ_DB")},
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

	// if err != nil { panic(err) }
	panicStmt := &ast.ExprStmt{X: &ast.CallExpr{Fun: Ident("panic"), Args: []ast.Expr{Ident("err")}}}
	ifErr := &ast.IfStmt{
		Cond: Binary(token.NEQ, Ident("err"), Ident("nil")),
		Body: &ast.BlockStmt{List: []ast.Stmt{panicStmt}},
	}

	// 6x reg.Provide(...)
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

	// Layout stamping
	packagePos := padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, registerPkg.Doc)
	registerPkg.Type.Func = padder.Take()
	registerPkg.Body.Lbrace = padder.Take()

	// Stamp each stmt; compose lit precisa StampCompositeLit pra sair multi-line.
	// loggerCall e dbCall ganham stampMultilineCallArgs pra quebrar args
	// (mesmo layout do bootstrap/http/pkg.go).
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

// buildQueueRepositoriesFile e buildQueueServicesFile geram stubs vazios
// idênticos em forma a bootstrap/http/{repositories,services}.go.
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

// buildQueueWorkerFile monta `bootstrap/<name>/worker.go` com
// `func NewHandler(reg *registry.Registry) omniq.ConsumeHandler` + stub.
func buildQueueWorkerFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-worker")

	imports := ImportGroups(padder,
		[]string{imp.join(registryImportSubpath)},
		[]string{omniqGoImportPath},
	)

	// _ = reg  // stub discard
	discardAssign := Assign(Ident("_"), Ident("reg"))

	// Closure: func(ctx omniq.JobCtx) { /* doc-only body */ }
	handlerFnType := &ast.FuncType{
		Params:  FieldList(Field("ctx", Sel("omniq", "JobCtx"))),
		Results: nil,
	}
	handlerLit := &ast.FuncLit{
		Type: handlerFnType,
		Body: &ast.BlockStmt{},
	}
	// Comentário dentro do corpo da closure como bloco de instruções pro dev.
	handlerComments := &ast.CommentGroup{List: []*ast.Comment{
		{Text: "// 1. decodificar payload com ctx.DecodePayload(&p)"},
		{Text: "// 2. chamar service(s) resolvidos via registry.Resolve no escopo externo"},
		{Text: "// 3. panic em erro irrecuperável → omniq marca o job como FAILED"},
	}}

	returnClosure := ReturnStmt(handlerLit)

	params := FieldList(Field("reg", StarOf(Sel("registry", "Registry"))))
	results := FieldList(AnonField(Sel("omniq", "ConsumeHandler")))
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

	// Layout stamping
	packagePos := padder.Take()
	padder.Gap(1)

	stampDocPositions(padder, fn.Doc)
	fn.Type.Func = padder.Take()
	fn.Body.Lbrace = padder.Take()
	stampAssignStmt(padder, discardAssign)
	returnClosure.Return = padder.Take()
	handlerLit.Type.Func = padder.Take()
	handlerLit.Body.Lbrace = padder.Take()
	// Stamp dos comentários do corpo da closure.
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

// buildQueueMainFile monta `cmd/<name>/main.go`: import log + alias do
// bootstrap, func main com if-err `log.Fatalf` em caso de erro do Run().
func buildQueueMainFile(name, bootAlias, bootImportPath string) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-queue-main")

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

	// Layout stamping
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

// --- helpers locais ---

// readConfigCall monta `conf.ReadConfig("<key>")`.
func readConfigCall(key string) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  Sel("conf", "ReadConfig"),
		Args: []ast.Expr{StrLit(key)},
	}
}

// readNumberConfigCall monta `conf.ReadNumberConfig("<key>")`.
func readNumberConfigCall(key string) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  Sel("conf", "ReadNumberConfig"),
		Args: []ast.Expr{StrLit(key)},
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
