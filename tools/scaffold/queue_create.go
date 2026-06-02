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
// vazio — quando os scaffolds existentes virarem entrypoint-aware (`--entrypoint`),
// continuam funcionando sem mudança nesses dois arquivos.
//
// Geração via string template + `format.Source`: o volume de 6 arquivos
// tornaria o AST-builder dominante. Output é byte-idêntico a gofmt e
// `format.Source` falha alto se o template tiver bug de sintaxe.
package scaffold

import (
	"fmt"
	"go/format"
	"path/filepath"
	"strings"
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
// ordem canônica (bootstrap setup → cmd main). Cada builder devolve source
// Go já formatado por `format.Source`; se algum template tiver bug de
// sintaxe, falha aqui antes de tocar o disco.
func buildQueueOmniqFiles(name string, imp importPaths) ([]queueFile, error) {
	bootAlias := stripUnderscores(name) + "boot"
	moduleBootstrap := imp.join(bootstrapBasePath + "/" + name)

	specs := []struct {
		rel string
		fn  func() string
	}{
		{filepath.Join(bootstrapBasePath, name, "setup.go"), func() string { return queueOmniqSetupSrc(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "configs.go"), func() string { return queueOmniqConfigsSrc(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "pkg.go"), func() string { return queueOmniqPkgSrc(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "repositories.go"), func() string { return queueOmniqRepositoriesSrc(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "services.go"), func() string { return queueOmniqServicesSrc(name, imp) }},
		{filepath.Join(bootstrapBasePath, name, "worker.go"), func() string { return queueOmniqWorkerSrc(name, imp) }},
		{filepath.Join(cmdBasePath, name, "main.go"), func() string { return queueOmniqMainSrc(bootAlias, moduleBootstrap) }},
	}

	out := make([]queueFile, 0, len(specs))
	for _, s := range specs {
		formatted, err := format.Source([]byte(s.fn()))
		if err != nil {
			return nil, fmt.Errorf("formatar %s: %w\n%s", s.rel, err, s.fn())
		}
		if !strings.HasSuffix(string(formatted), "\n") {
			formatted = append(formatted, '\n')
		}
		out = append(out, queueFile{relPath: s.rel, src: formatted})
	}
	return out, nil
}

// queueOmniqSetupSrc — bootstrap/<name>/setup.go.
func queueOmniqSetupSrc(name string, imp importPaths) string {
	return fmt.Sprintf(`// Package %[1]s wires the %[1]s queue worker entrypoint (omniq driver) and
// exposes Run() to drive the consume loop. Layout mirrors bootstrap/http/:
// configs.go loads envs, pkg.go registers primitives (logger, db, omniq
// client), repositories.go and services.go are entrypoint-agnostic wire-up,
// worker.go bridges omniq jobs to services via NewHandler(reg).
package %[1]s

import (
	"%[2]s"

	"%[3]s"
)

// omniqClientRegistryKey é a chave do *omniq.Client no registry. Privada ao
// pacote do bootstrap porque só este entrypoint consome o client.
const omniqClientRegistryKey = "omniq.client"

// Setup loads configs and registers primitives, repositories and services
// into the registry in topological order. Returns (registry, queueName).
// Does NOT start the consume loop — use Run() for that. Setup() é separado
// pra que testes e scripts ad-hoc consigam inspecionar o grafo sem assinar
// uma fila.
func Setup() (reg *registry.Registry, queueName string) {
	conf, queueName := loadConfigs()
	reg = registry.NewRegistry()
	registerPkg(reg, conf)
	registerRepositories(reg)
	registerServices(reg)
	return reg, queueName
}

// Run is the long-running entrypoint: wires the graph via Setup() and starts
// the omniq consume loop. Blocks until omniq.Consume returns. One process ==
// one queue == one worker — crash semantics são intencionais, o orquestrador
// (k8s/systemd) reinicia em falha.
func Run() error {
	reg, queueName := Setup()
	client := registry.Resolve[*omniq.Client](reg, omniqClientRegistryKey)
	return client.Consume(omniq.ConsumeOpts{
		Queue:   queueName,
		Handler: NewHandler(reg),
		Verbose: true,
		Drain:   true,
	})
}
`, name, imp.join(registryImportSubpath), omniqGoImportPath)
}

// queueOmniqConfigsSrc — bootstrap/<name>/configs.go.
func queueOmniqConfigsSrc(name string, imp importPaths) string {
	return fmt.Sprintf(`package %[1]s

import (
	"%[2]s"
)

// loadConfigs reads the envs and returns the config instance plus the queue
// name. queueName sai por valor porque Run() precisa dele pra chamar
// omniq.Consume; o restante dos envs (host/port/db, primitivos) é lido sob
// demanda em pkg.go via *config.Config.
func loadConfigs() (conf *config.Config, queueName string) {
	conf = config.NewConfig()
	if err := conf.LoadEnvs(); err != nil {
		panic(err)
	}
	queueName = conf.ReadConfig("OMNIQ_QUEUE")
	return conf, queueName
}
`, name, imp.join("pkg/config"))
}

// queueOmniqPkgSrc — bootstrap/<name>/pkg.go.
func queueOmniqPkgSrc(name string, imp importPaths) string {
	return fmt.Sprintf(`package %[1]s

import (
	"%[2]s"
	"%[3]s"
	"%[4]s"
	"%[5]s"
	"%[6]s"
	"%[7]s"

	"%[8]s"
)

// registerPkg registers the primitive dependencies: logger, validator, config,
// idCreator, db (*sqlx.DB) e o *omniq.Client. Depende apenas dos configs já
// carregados. Falha rápida no boot se algum env obrigatório estiver ausente.
func registerPkg(reg *registry.Registry, conf *config.Config) {
	l := logger.NewLogger(
		conf.ReadConfig("ENVIRONMENT"),
		conf.ReadConfig("APP"),
		conf.ReadConfig("VERSION"),
	)
	l.Boot()

	db := database.NewMysql(
		l,
		conf.ReadConfig("DB_USER"),
		conf.ReadConfig("DB_PASS"),
		conf.ReadConfig("DB_URL"),
		conf.ReadConfig("DB_PORT"),
		conf.ReadConfig("DB_DATABASE"),
	)
	db.Connect()

	val := validator.NewValidator()
	val.Boot()

	idC := idCreator.NewIdCreator()

	client, err := omniq.NewClient(omniq.ClientOpts{
		Host: conf.ReadConfig("OMNIQ_HOST"),
		Port: conf.ReadNumberConfig("OMNIQ_PORT"),
		DB:   conf.ReadNumberConfig("OMNIQ_DB"),
	})
	if err != nil {
		panic(err)
	}

	reg.Provide(logger.RegistryKey, l)
	reg.Provide(validator.RegistryKey, val)
	reg.Provide(config.RegistryKey, conf)
	reg.Provide(idCreator.RegistryKey, idC)
	reg.Provide(database.RegistryKey, db.Db)
	reg.Provide(omniqClientRegistryKey, client)
}
`,
		name,
		imp.join("pkg/config"),
		imp.join("pkg/database"),
		imp.join("pkg/idCreator"),
		imp.join("pkg/logger"),
		imp.join(registryImportSubpath),
		imp.join("pkg/validator"),
		omniqGoImportPath,
	)
}

// queueOmniqRepositoriesSrc — bootstrap/<name>/repositories.go.
// Corpo vazio inicial; a futura `repository register --entrypoint <name>`
// vai apender Provides aqui.
func queueOmniqRepositoriesSrc(name string, imp importPaths) string {
	return fmt.Sprintf(`package %[1]s

import (
	"%[2]s"
)

// registerRepositories registers the persistence repositories. Depende apenas
// de db (*sqlx.DB) já registrado por pkg.go. A scaffold tool apende novos
// repositórios aqui.
func registerRepositories(_ *registry.Registry) {
}
`, name, imp.join(registryImportSubpath))
}

// queueOmniqServicesSrc — bootstrap/<name>/services.go.
func queueOmniqServicesSrc(name string, imp importPaths) string {
	return fmt.Sprintf(`package %[1]s

import (
	"%[2]s"
)

// registerServices builds and registers the application services (use cases)
// into the registry. Services resolvem primitivos e repositórios do registry
// e são construídos eagermente, então uma dep faltando falha rápido no boot.
// A scaffold tool apende novos services aqui.
func registerServices(_ *registry.Registry) {
}
`, name, imp.join(registryImportSubpath))
}

// queueOmniqWorkerSrc — bootstrap/<name>/worker.go.
func queueOmniqWorkerSrc(name string, imp importPaths) string {
	return fmt.Sprintf(`package %[1]s

import (
	"%[2]s"

	"%[3]s"
)

// NewHandler constrói o ConsumeHandler do worker. Resolva services/repos do
// registry aqui (closure) e use-os dentro do handler retornado. Não é
// registrado no registry porque há apenas um worker por entrypoint — Run()
// constrói direto.
//
// Contrato omniq: handler retornando normalmente == job ACKed; panic ==
// job marcado como FAILED e candidato a retry (até MaxAttempts). Use
// ctx.DecodePayload(&p) pra extrair o payload tipado.
func NewHandler(reg *registry.Registry) omniq.ConsumeHandler {
	_ = reg // remova quando resolver o primeiro service aqui
	return func(ctx omniq.JobCtx) {
		// 1. decodificar payload com ctx.DecodePayload(&p)
		// 2. chamar service(s) resolvidos via registry.Resolve no escopo externo
		// 3. panic em erro irrecuperável → omniq marca o job como FAILED
	}
}
`, name, imp.join(registryImportSubpath), omniqGoImportPath)
}

// queueOmniqMainSrc — cmd/<name>/main.go.
func queueOmniqMainSrc(bootAlias, bootImportPath string) string {
	return fmt.Sprintf(`// Command worker é o entrypoint de worker (esqueleto gerado por scaffold queue create).
package main

import (
	"log"

	%[1]s "%[2]s"
)

func main() {
	if err := %[1]s.Run(); err != nil {
		log.Fatalf("queue worker: %%v", err)
	}
}
`, bootAlias, bootImportPath)
}
