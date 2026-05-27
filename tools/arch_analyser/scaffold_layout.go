// Package arch_analyser — validador de layout de arquivos (NAVE-126).
//
// ValidateScaffoldLayout faz enforce HARD da estrutura de camadas: só pode
// existir no projeto exatamente o shape que o scaffold gera. Qualquer arquivo
// fora do padrão é violação — validação do filesystem, não só do grafo de
// imports (que continua coberto por ValidateNoHandlerCrossImports, decisão 5).
//
// Modo report: a função agrega TODAS as violações numa única mensagem de erro,
// sem abortar no primeiro arquivo divergente. Quem consome (CLI/MCP) decide se
// derruba o build; nesta fase o validador entra report-only até o repo estar
// conforme (ver proposta NAVE-126).
//
// Decisão central (proposta): o analyser NÃO importa nada do scaffold. Os paths
// canônicos são duplicados aqui como constantes/allowlists. A divergência
// scaffold↔analyser é a única rede de segurança, coberta pelo teste de
// consistência (CP5).
package arch_analyser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Allowlists fixas — paths de infra que o scaffold não gera por domínio, mas
// que são parte legítima e imutável do esqueleto. Ficam como constantes aqui
// (decisão: não criar pacote "layout" compartilhado).
// ---------------------------------------------------------------------------

// bootstrapAllowedFiles é o conjunto fechado de arquivos de bootstrap/. Nada
// novo entra sem virar violação.
var bootstrapAllowedFiles = map[string]struct{}{
	"configs.go":      {},
	"handlers.go":     {},
	"pkg.go":          {},
	"repositories.go": {},
	"services.go":     {},
	"setup.go":        {},
}

// routesAllowedExtraFiles são os arquivos de cmd/http/routes/ que não seguem o
// shape <domain>.go: o agregador declarable.go e a rota sem domínio health.go.
var routesAllowedExtraFiles = map[string]struct{}{
	"declarable.go": {},
	"health.go":     {},
}

// scaffoldLayoutViolation é uma divergência catalogada pelo validador.
type scaffoldLayoutViolation struct {
	path string // path relativo à raiz do repo
	rule string // regra violada (camada)
	msg  string
}

func (v scaffoldLayoutViolation) String() string {
	return fmt.Sprintf("%s: [%s] %s", v.path, v.rule, v.msg)
}

// layoutChecker acumula o estado de uma execução do validador.
type layoutChecker struct {
	root       string
	violations []scaffoldLayoutViolation
}

func (c *layoutChecker) add(path, rule, msg string) {
	rel, err := filepath.Rel(c.root, path)
	if err != nil {
		rel = path
	}
	c.violations = append(c.violations, scaffoldLayoutViolation{
		path: filepath.ToSlash(rel),
		rule: rule,
		msg:  msg,
	})
}

// ValidateScaffoldLayout percorre as camadas e falha em qualquer arquivo fora
// do shape do scaffold. Report-mode: agrega todas as violações.
func ValidateScaffoldLayout(root string) error {
	c := &layoutChecker{root: root}

	c.checkBootstrap()
	c.checkHandlers()
	c.checkRoutes()
	c.checkMiddlewares()
	c.checkServices()
	c.checkDomain()
	c.checkRepositories()

	if len(c.violations) == 0 {
		return nil
	}
	sort.Slice(c.violations, func(i, j int) bool {
		if c.violations[i].path != c.violations[j].path {
			return c.violations[i].path < c.violations[j].path
		}
		return c.violations[i].msg < c.violations[j].msg
	})
	lines := make([]string, 0, len(c.violations))
	for _, v := range c.violations {
		lines = append(lines, v.String())
	}
	return fmt.Errorf("validação de layout do scaffold falhou (%d violação(ões)):\n%s",
		len(c.violations), strings.Join(lines, "\n"))
}

// checkBootstrap valida bootstrap/ contra o conjunto fechado de arquivos.
func (c *layoutChecker) checkBootstrap() {
	dir := filepath.Join(c.root, "bootstrap")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // bootstrap ausente não é alvo deste validador
	}
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			c.add(full, "bootstrap", "subdiretório não permitido em bootstrap/ (conjunto fechado de arquivos)")
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if _, ok := bootstrapAllowedFiles[e.Name()]; !ok {
			c.add(full, "bootstrap", "arquivo fora do conjunto fechado de bootstrap/ (configs/handlers/pkg/repositories/services/setup)")
		}
	}
}

// checkHandlers valida cmd/http/handlers/ no shape estrito do scaffold:
// só cmd/http/handlers/<domain>/<verb>/handler.go.
//
//   - nada solto no nível do domínio (handlers/<d>/x.go é violação);
//   - nenhum subpacote auxiliar dentro do verbo nem como verbo
//     (handlers/<d>/cookie/, handlers/<d>/<v>/sub/ são violação — mata
//     cookie/ e a classe do ownership.go, decisão da NAVE-126);
//   - único arquivo permitido no verbo é handler.go (+ handler_test.go);
//   - 1:1 handler→service: todo handlers/<d>/<v>/ exige
//     services/<d>/<v>/ (a regra é direcional — service pode existir sem
//     handler, ex.: usage_record/export, ver proposta).
func (c *layoutChecker) checkHandlers() {
	handlersRoot := filepath.Join(c.root, "cmd", "http", "handlers")
	servicesRoot := filepath.Join(c.root, "internal", "application", "services")
	if _, err := os.Stat(handlersRoot); err != nil {
		return
	}
	for _, domain := range listSubdirs(handlersRoot) {
		domainDir := filepath.Join(handlersRoot, domain)

		// Arquivos .go soltos no nível do domínio são violação.
		for _, f := range listGoFiles(domainDir) {
			c.add(f, "handlers", "arquivo no nível do domínio — handler deve viver em handlers/<domain>/<verb>/handler.go")
		}

		for _, verb := range listSubdirs(domainDir) {
			verbDir := filepath.Join(domainDir, verb)

			// Subpacote dentro do verbo é proibido (sem auxiliar horizontal).
			for _, sub := range listSubdirs(verbDir) {
				c.add(filepath.Join(verbDir, sub), "handlers",
					"subpacote não permitido dentro do verbo — handler é estritamente handlers/<domain>/<verb>/handler.go")
			}

			// Único arquivo permitido no verbo: handler.go.
			for _, f := range listGoFiles(verbDir) {
				if filepath.Base(f) != "handler.go" {
					c.add(f, "handlers", "arquivo extra no verbo — só handler.go é permitido em handlers/<domain>/<verb>/")
				}
			}

			// 1:1 handler→service: o verbo precisa ter service associado.
			svcDir := filepath.Join(servicesRoot, domain, verb)
			if _, err := os.Stat(svcDir); err != nil {
				c.add(verbDir, "handlers",
					fmt.Sprintf("handler sem service 1:1 — esperado services/%s/%s/", domain, verb))
			}
		}
	}
}

// domainNames retorna o set de nomes de domínio existentes em
// internal/application/domain/<nome>/. Usado para cruzar com as camadas que
// referenciam o domínio por nome (routes, repositories).
func (c *layoutChecker) domainNames() map[string]struct{} {
	names := make(map[string]struct{})
	for _, d := range listSubdirs(filepath.Join(c.root, "internal", "application", "domain")) {
		names[d] = struct{}{}
	}
	return names
}

// checkRoutes valida cmd/http/routes/: só <domain>.go + declarable.go +
// health.go (allowlist), sem subdiretórios. Cada arquivo que não está na
// allowlist precisa casar com um domínio existente (<domain>.go).
func (c *layoutChecker) checkRoutes() {
	dir := filepath.Join(c.root, "cmd", "http", "routes")
	if _, err := os.Stat(dir); err != nil {
		return
	}
	for _, sub := range listSubdirs(dir) {
		c.add(filepath.Join(dir, sub), "routes", "subdiretório não permitido em routes/ (só <domain>.go + declarable.go + health.go)")
	}
	domains := c.domainNames()
	for _, f := range listGoFiles(dir) {
		base := filepath.Base(f)
		if _, ok := routesAllowedExtraFiles[base]; ok {
			continue
		}
		name := strings.TrimSuffix(base, ".go")
		if _, ok := domains[name]; !ok {
			c.add(f, "routes", "arquivo de rota sem domínio associado — esperado <domain>.go com internal/application/domain/<domain>/ ou um dos permitidos (declarable.go, health.go)")
		}
	}
}

// checkMiddlewares valida cmd/http/middlewares/: gate de aplicação único e
// parametrizável (decisão NAVE-126). A infra transversal vive em
// cmd/http/server/server.go via e.Use() e fica FORA do enforce. Aqui o
// validador conta os arquivos .go de gate: mais de um arquivo indica
// proliferação de MiddlewareFunc de negócio ad-hoc.
func (c *layoutChecker) checkMiddlewares() {
	dir := filepath.Join(c.root, "cmd", "http", "middlewares")
	if _, err := os.Stat(dir); err != nil {
		return
	}
	for _, sub := range listSubdirs(dir) {
		c.add(filepath.Join(dir, sub), "middleware", "subdiretório não permitido em middlewares/ (gate de aplicação único e parametrizável)")
	}
	gateFiles := listGoFiles(dir)
	if len(gateFiles) > 1 {
		sort.Strings(gateFiles)
		for _, f := range gateFiles {
			c.add(f, "middleware",
				fmt.Sprintf("proliferação de gate de aplicação — esperado 1 gate canônico parametrizável em middlewares/, encontrados %d arquivos", len(gateFiles)))
		}
	}
}

// serviceTrioFiles é o shape canônico de um verbo de service (NAVE-124).
var serviceTrioFiles = map[string]struct{}{
	"request.go":  {},
	"service.go":  {},
	"response.go": {},
}

// servicesRootAllowedFiles são os arquivos permitidos na raiz de services/
// (base compartilhada gerada pelo template, não por domínio). errors.go
// carrega o AppError e os construtores de erro de aplicação (NAVE-135),
// base compartilhada como base_service.go.
var servicesRootAllowedFiles = map[string]struct{}{
	"base_service.go": {},
	"errors.go":       {},
}

// checkServices valida internal/application/services/: cada verbo precisa do
// trio request.go/service.go/response.go (service.go solo é violação). Sem
// subpacote dentro do verbo nem arquivo fora do trio. base_service.go é
// permitido na raiz.
func (c *layoutChecker) checkServices() {
	root := filepath.Join(c.root, "internal", "application", "services")
	if _, err := os.Stat(root); err != nil {
		return
	}
	// Arquivos soltos na raiz de services/: só base_service.go.
	for _, f := range listGoFiles(root) {
		if _, ok := servicesRootAllowedFiles[filepath.Base(f)]; !ok {
			c.add(f, "service", "arquivo solto na raiz de services/ — só base_service.go é permitido")
		}
	}
	for _, domain := range listSubdirs(root) {
		domainDir := filepath.Join(root, domain)
		// Arquivos soltos no nível do domínio (fora de um verbo) são violação.
		for _, f := range listGoFiles(domainDir) {
			c.add(f, "service", "arquivo no nível do domínio — service deve viver em services/<domain>/<verb>/")
		}
		for _, verb := range listSubdirs(domainDir) {
			c.checkServiceVerb(domain, verb, filepath.Join(domainDir, verb))
		}
	}
}

// checkServiceVerb valida um único verbo de service contra o trio canônico.
func (c *layoutChecker) checkServiceVerb(domain, verb, verbDir string) {
	for _, sub := range listSubdirs(verbDir) {
		c.add(filepath.Join(verbDir, sub), "service",
			"subpacote não permitido dentro do verbo — service é o trio request.go/service.go/response.go")
	}
	present := make(map[string]struct{})
	for _, f := range listGoFiles(verbDir) {
		base := filepath.Base(f)
		present[base] = struct{}{}
		if _, ok := serviceTrioFiles[base]; !ok {
			c.add(f, "service", "arquivo fora do trio — só request.go/service.go/response.go em services/<domain>/<verb>/")
		}
	}
	for want := range serviceTrioFiles {
		if _, ok := present[want]; !ok {
			c.add(verbDir, "service",
				fmt.Sprintf("trio incompleto — falta %s em services/%s/%s/", want, domain, verb))
		}
	}
}

// checkDomain valida internal/application/domain/: cada pasta só pode ter
// <domain>.go. ports.go/errors.go viram violação (decisão 2 — conteúdo vai
// pro <domain>.go).
func (c *layoutChecker) checkDomain() {
	root := filepath.Join(c.root, "internal", "application", "domain")
	if _, err := os.Stat(root); err != nil {
		return
	}
	for _, domain := range listSubdirs(root) {
		domainDir := filepath.Join(root, domain)
		for _, sub := range listSubdirs(domainDir) {
			c.add(filepath.Join(domainDir, sub), "domain", "subdiretório não permitido em domain/<domain>/ (só <domain>.go)")
		}
		for _, f := range listGoFiles(domainDir) {
			if filepath.Base(f) != domain+".go" {
				c.add(f, "domain",
					fmt.Sprintf("arquivo fora do shape — só %s.go é permitido em domain/%s/ (ports.go/errors.go vão pro <domain>.go)", domain, domain))
			}
		}
	}
}

// repoBaseDirs são os subdiretórios de internal/repositories/ que não seguem o
// shape <domain>/<domain>.go (base compartilhada gerada pelo template).
var repoBaseDirs = map[string]struct{}{
	"base_repository": {},
}

// checkRepositories valida internal/repositories/: cada pasta de domínio só
// pode ter <domain>.go, e o domínio precisa existir em domain/ (repo sem
// domain correspondente é violação — decisão 3). repository.go é violação
// (decisão 1). base_repository/ é permitido.
func (c *layoutChecker) checkRepositories() {
	root := filepath.Join(c.root, "internal", "repositories")
	if _, err := os.Stat(root); err != nil {
		return
	}
	// Arquivos soltos na raiz de repositories/ são violação.
	for _, f := range listGoFiles(root) {
		c.add(f, "repository", "arquivo solto na raiz de repositories/ — repo deve viver em repositories/<domain>/<domain>.go")
	}
	domains := c.domainNames()
	for _, name := range listSubdirs(root) {
		if _, ok := repoBaseDirs[name]; ok {
			continue
		}
		dir := filepath.Join(root, name)
		if _, ok := domains[name]; !ok {
			c.add(dir, "repository",
				fmt.Sprintf("repositório sem domain associado — esperado internal/application/domain/%s/ (decisão 3)", name))
		}
		for _, sub := range listSubdirs(dir) {
			c.add(filepath.Join(dir, sub), "repository", "subdiretório não permitido em repositories/<domain>/ (só <domain>.go)")
		}
		for _, f := range listGoFiles(dir) {
			if filepath.Base(f) != name+".go" {
				c.add(f, "repository",
					fmt.Sprintf("arquivo fora do shape — só %s.go é permitido em repositories/%s/ (repository.go vira violação, decisão 1)", name, name))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers AST compartilhados pelas regras de camada.
// ---------------------------------------------------------------------------

// listGoFiles retorna os arquivos .go (não-test) diretamente dentro de dir
// (não-recursivo). Retorna nil se dir não existir.
func listGoFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out
}

// listSubdirs retorna os nomes dos subdiretórios diretos de dir. nil se ausente.
func listSubdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}
