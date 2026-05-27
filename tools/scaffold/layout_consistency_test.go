package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	arch "github.com/Open-Zord/zord/tools/arch_analyser"
)

// Teste de consistência scaffold ↔ analyser.
//
// Garante a invariante central da proposta: TUDO que o scaffold gera passa no
// ValidateScaffoldLayout, e qualquer divergência conhecida (subpacote auxiliar
// em handlers, arquivo fora do trio em service, 2º gate de aplicação) é
// reprovada. É a única rede de segurança contra a divergência de paths — a
// decisão travada foi NÃO compartilhar um pacote "layout"; o analyser duplica
// as strings de path, e este teste é quem pega quando scaffold e analyser
// saem de sincronia.
//
// Decisão de design: TempDir hermético, NÃO a worktree real.
//   - A worktree real tem variâncias pré-existentes (repos com nome divergente,
//     ports.go/errors.go, repos sem domain, services solo, cookie/), então o
//     analyser nunca retornaria "passa" lá — inútil pra afirmar a invariante
//     "o que o scaffold gera é conforme".
//   - Gerar um domínio descartável na worktree real arriscaria deixar
//     artefatos órfãos se o teste falhasse no meio. O TempDir é hermético,
//     seguro pra rodar em paralelo, e o próprio testing.T limpa no fim.
//   - O scaffold não exige go.mod nem a estrutura completa do repo pra rodar
//     (faz edição AST sobre fixtures mínimas), então a estrutura mínima
//     conforme cabe num TempDir. Os seeders do próprio pacote scaffold
//     (seedBootstrap*, seedDeclarable, seedRouteFile) montam o esqueleto.
//
// O analyser NÃO importa o scaffold (decisão travada); este teste vive no
// pacote scaffold e importa o analyser — o fluxo de dependência permitido.

// seedMinimalConformingRepo monta, num TempDir, o esqueleto mínimo conforme:
// conjunto fechado de bootstrap/, declarable.go + health.go em routes/. Não
// cria nenhum domínio — isso fica a cargo do scaffold.
func seedMinimalConformingRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// go.mod mínimo (alguns fluxos lêem o module path; inofensivo se não).
	writeFileTree(t, root, map[string]string{
		"go.mod": "module zord\n\ngo 1.22\n",
		// bootstrap: configs/pkg/setup planos; services/repositories/handlers
		// vêm dos seeders abaixo (precisam do shape que os register editam).
		"bootstrap/configs.go": "package bootstrap\n",
		"bootstrap/pkg.go":     "package bootstrap\n",
		"bootstrap/setup.go":   "package bootstrap\n",
		// routes: health.go (rota sem domínio, allowlist) + declarable.go.
		"cmd/http/routes/health.go": "package routes\n\nfunc NewHealthRoute() Declarable { return nil }\n",
	})
	seedBootstrapServices(t, root)
	seedBootstrapRepositories(t, root)
	seedBootstrapHandlers(t, root)
	seedDeclarable(t, root, declarableSemEntradas)
	return root
}

// scaffoldProbeDomain roda a cadeia completa do scaffold pra um domínio
// descartável "Probe" com um verbo "Fetch", cobrindo todas as camadas que o
// analyser valida: domain, repository (port + create), service (trio), handler
// e rota (create + add + register dos três).
func scaffoldProbeDomain(t *testing.T, root string) {
	t.Helper()
	const domain = "Probe"
	const verb = "Fetch"

	if _, err := DomainCreate(domain, DomainCreateOptions{Root: root}); err != nil {
		t.Fatalf("DomainCreate: %v", err)
	}
	if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: domain, FieldName: "Name", TypeStr: "string"}); err != nil {
		t.Fatalf("FieldAdd: %v", err)
	}
	if _, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: domain}); err != nil {
		t.Fatalf("RepositoryPort: %v", err)
	}
	if _, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: domain}); err != nil {
		t.Fatalf("RepositoryCreate: %v", err)
	}
	if _, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: domain, Verb: verb}); err != nil {
		t.Fatalf("ServiceCreate: %v", err)
	}
	if _, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: domain, Service: verb}); err != nil {
		t.Fatalf("HandlerCreate: %v", err)
	}
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: domain}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: domain, Service: verb, Method: "GET"}); err != nil {
		t.Fatalf("RouteAdd: %v", err)
	}
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: domain, Verb: verb}); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}
	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: domain}); err != nil {
		t.Fatalf("RegisterRepository: %v", err)
	}
	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: domain, Service: verb}); err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}
	if _, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: domain}); err != nil {
		t.Fatalf("RouteRegister: %v", err)
	}
}

func TestScaffoldLayoutConsistency_GeradoPeloScaffoldPassa(t *testing.T) {
	root := seedMinimalConformingRepo(t)
	scaffoldProbeDomain(t, root)

	if err := arch.ValidateScaffoldLayout(root); err != nil {
		t.Fatalf("domínio gerado pelo scaffold deveria PASSAR no analyser, mas reprovou:\n%v", err)
	}
}

func TestScaffoldLayoutConsistency_ViolacoesInjetadasReprovam(t *testing.T) {
	t.Run("subpacote auxiliar em handlers", func(t *testing.T) {
		root := seedMinimalConformingRepo(t)
		scaffoldProbeDomain(t, root)
		writeFileTree(t, root, map[string]string{
			"cmd/http/handlers/probe/shared/util.go": "package shared\n",
		})
		err := arch.ValidateScaffoldLayout(root)
		if err == nil || !strings.Contains(err.Error(), "cmd/http/handlers/probe/shared") {
			t.Fatalf("esperava reprovação do subpacote auxiliar em handlers: %v", err)
		}
	})

	t.Run("arquivo fora do trio no service", func(t *testing.T) {
		root := seedMinimalConformingRepo(t)
		scaffoldProbeDomain(t, root)
		writeFileTree(t, root, map[string]string{
			"internal/application/services/probe/fetch/helper.go": "package fetch\n",
		})
		err := arch.ValidateScaffoldLayout(root)
		if err == nil || !strings.Contains(err.Error(), "fora do trio") {
			t.Fatalf("esperava reprovação de arquivo fora do trio: %v", err)
		}
	})

	t.Run("2o gate de aplicacao", func(t *testing.T) {
		root := seedMinimalConformingRepo(t)
		scaffoldProbeDomain(t, root)
		// o scaffold é agnóstico a middleware; injetamos dois gates à mão.
		writeFileTree(t, root, map[string]string{
			"cmd/http/middlewares/auth_jwt.go": "package middlewares\n",
			"cmd/http/middlewares/rbac.go":     "package middlewares\n",
		})
		err := arch.ValidateScaffoldLayout(root)
		if err == nil || !strings.Contains(err.Error(), "proliferação de gate") {
			t.Fatalf("esperava reprovação de 2º gate de aplicação: %v", err)
		}
	})
}

// sanity: o helper de cleanup explícito não é necessário (t.TempDir limpa), mas
// deixamos um guard contra vazamento de artefato fora do TempDir caso alguém
// futuramente troque a fonte do root.
func TestScaffoldLayoutConsistency_RootEhTempDir(t *testing.T) {
	root := seedMinimalConformingRepo(t)
	if !strings.HasPrefix(root, os.TempDir()) {
		t.Fatalf("root da consistência deveria estar sob TempDir, got %q", filepath.Clean(root))
	}
}
