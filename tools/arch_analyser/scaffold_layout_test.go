package arch_analyser

import (
	"strings"
	"testing"
)

// bootstrapOK monta o conjunto fechado de bootstrap/ esperado pelo scaffold.
func bootstrapOK() map[string]string {
	return map[string]string{
		"bootstrap/configs.go":      "package bootstrap\n",
		"bootstrap/handlers.go":     "package bootstrap\n",
		"bootstrap/pkg.go":          "package bootstrap\n",
		"bootstrap/repositories.go": "package bootstrap\n",
		"bootstrap/services.go":     "package bootstrap\n",
		"bootstrap/setup.go":        "package bootstrap\n",
	}
}

func TestValidateScaffoldLayout_Bootstrap_OK(t *testing.T) {
	root := setupFakeRepo(t, bootstrapOK())
	if err := ValidateScaffoldLayout(root); err != nil {
		t.Fatalf("esperava sucesso, deu erro: %v", err)
	}
}

func TestValidateScaffoldLayout_Bootstrap_ArquivoExtra(t *testing.T) {
	files := bootstrapOK()
	files["bootstrap/extra.go"] = "package bootstrap\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil {
		t.Fatal("esperava erro por arquivo extra em bootstrap/, mas passou")
	}
	if !strings.Contains(err.Error(), "bootstrap/extra.go") {
		t.Fatalf("erro deveria citar o arquivo extra: %v", err)
	}
	if !strings.Contains(err.Error(), "[bootstrap]") {
		t.Fatalf("erro deveria identificar a regra bootstrap: %v", err)
	}
}

func TestValidateScaffoldLayout_Bootstrap_Subdiretorio(t *testing.T) {
	files := bootstrapOK()
	files["bootstrap/sub/x.go"] = "package sub\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil {
		t.Fatal("esperava erro por subdiretório em bootstrap/, mas passou")
	}
	if !strings.Contains(err.Error(), "bootstrap/sub") {
		t.Fatalf("erro deveria citar o subdiretório: %v", err)
	}
}

func TestValidateScaffoldLayout_Bootstrap_TestFileIgnorado(t *testing.T) {
	files := bootstrapOK()
	files["bootstrap/setup_test.go"] = "package bootstrap\n"
	root := setupFakeRepo(t, files)
	if err := ValidateScaffoldLayout(root); err != nil {
		t.Fatalf("arquivo _test.go não deveria virar violação: %v", err)
	}
}

func TestValidateScaffoldLayout_SemBootstrap_OK(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{"main.go": "package main\n"})
	if err := ValidateScaffoldLayout(root); err != nil {
		t.Fatalf("repo sem bootstrap/ não deveria falhar: %v", err)
	}
}

// ----- CP2: handlers -----

// serviceTrio devolve os três arquivos do trio para um verbo.
func serviceTrio(files map[string]string, domain, verb string) {
	base := "internal/application/services/" + domain + "/" + verb + "/"
	files[base+"request.go"] = "package " + verb + "\n"
	files[base+"service.go"] = "package " + verb + "\n"
	files[base+"response.go"] = "package " + verb + "\n"
}

// handlersOK monta um par handler/service 1:1 válido pra dois verbos, com o
// trio de service completo (pra não disparar a regra de service do CP3).
func handlersOK() map[string]string {
	files := bootstrapOK()
	serviceTrio(files, "foo", "create")
	serviceTrio(files, "foo", "list")
	files["cmd/http/handlers/foo/create/handler.go"] = "package create\n\ntype CreateHandler struct{}\n"
	files["cmd/http/handlers/foo/list/handler.go"] = "package list\n\ntype ListHandler struct{}\n"
	return files
}

func TestValidateScaffoldLayout_Handlers_OK(t *testing.T) {
	root := setupFakeRepo(t, handlersOK())
	if err := ValidateScaffoldLayout(root); err != nil {
		t.Fatalf("esperava sucesso, deu erro: %v", err)
	}
}

func TestValidateScaffoldLayout_Handlers_ArquivoNoNivelDoDominio(t *testing.T) {
	files := handlersOK()
	files["cmd/http/handlers/foo/handler.go"] = "package foo\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil {
		t.Fatal("esperava erro por arquivo no nível do domínio, mas passou")
	}
	if !strings.Contains(err.Error(), "cmd/http/handlers/foo/handler.go") ||
		!strings.Contains(err.Error(), "nível do domínio") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestValidateScaffoldLayout_Handlers_ArquivoExtraNoVerbo(t *testing.T) {
	files := handlersOK()
	files["cmd/http/handlers/foo/create/util.go"] = "package create\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil {
		t.Fatal("esperava erro por arquivo extra no verbo, mas passou")
	}
	if !strings.Contains(err.Error(), "cmd/http/handlers/foo/create/util.go") {
		t.Fatalf("erro deveria citar o arquivo extra: %v", err)
	}
}

func TestValidateScaffoldLayout_Handlers_SubpacoteNoVerbo(t *testing.T) {
	files := handlersOK()
	files["cmd/http/handlers/foo/create/inner/x.go"] = "package inner\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil {
		t.Fatal("esperava erro por subpacote no verbo, mas passou")
	}
	if !strings.Contains(err.Error(), "cmd/http/handlers/foo/create/inner") ||
		!strings.Contains(err.Error(), "subpacote") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestValidateScaffoldLayout_Handlers_SemService1a1(t *testing.T) {
	files := handlersOK()
	// handler sem service associado
	files["cmd/http/handlers/foo/orphan/handler.go"] = "package orphan\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil {
		t.Fatal("esperava erro por handler sem service 1:1, mas passou")
	}
	if !strings.Contains(err.Error(), "handler sem service 1:1") ||
		!strings.Contains(err.Error(), "services/foo/orphan/") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

// ----- CP3: routes, middleware, service (trio), domain, repository -----

// fullOK monta um repo mínimo conforme em TODAS as camadas: domain foo,
// repository foo, trio de service foo/create, handler foo/create 1:1, rota
// foo.go + declarable.go + health.go, gate de aplicação único.
func fullOK() map[string]string {
	files := bootstrapOK()
	files["internal/application/domain/foo/foo.go"] = "package foo\n"
	files["internal/repositories/foo/foo.go"] = "package foo\n"
	files["internal/application/services/base_service.go"] = "package services\n"
	files["internal/application/services/foo/create/request.go"] = "package create\n"
	files["internal/application/services/foo/create/service.go"] = "package create\n"
	files["internal/application/services/foo/create/response.go"] = "package create\n"
	files["cmd/http/handlers/foo/create/handler.go"] = "package create\n\ntype CreateHandler struct{}\n"
	files["cmd/http/routes/foo.go"] = "package routes\n"
	files["cmd/http/routes/declarable.go"] = "package routes\n"
	files["cmd/http/routes/health.go"] = "package routes\n"
	files["cmd/http/middlewares/auth_jwt.go"] = "package middlewares\n"
	return files
}

func TestValidateScaffoldLayout_Full_OK(t *testing.T) {
	root := setupFakeRepo(t, fullOK())
	if err := ValidateScaffoldLayout(root); err != nil {
		t.Fatalf("repo conforme não deveria falhar: %v", err)
	}
}

func TestValidateScaffoldLayout_Routes_SemDominio(t *testing.T) {
	files := fullOK()
	files["cmd/http/routes/ghost.go"] = "package routes\n" // sem domain ghost
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "cmd/http/routes/ghost.go") {
		t.Fatalf("esperava violação de rota sem domínio: %v", err)
	}
}

func TestValidateScaffoldLayout_Middleware_Proliferacao(t *testing.T) {
	files := fullOK()
	files["cmd/http/middlewares/rbac.go"] = "package middlewares\n" // 2o gate ad-hoc
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "proliferação de gate") {
		t.Fatalf("esperava violação de proliferação de gate: %v", err)
	}
}

func TestValidateScaffoldLayout_Service_TrioIncompleto(t *testing.T) {
	files := fullOK()
	// service.go solo: remove request/response do trio
	delete(files, "internal/application/services/foo/create/request.go")
	delete(files, "internal/application/services/foo/create/response.go")
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "trio incompleto") {
		t.Fatalf("esperava violação de trio incompleto: %v", err)
	}
}

func TestValidateScaffoldLayout_Service_ArquivoForaDoTrio(t *testing.T) {
	files := fullOK()
	files["internal/application/services/foo/create/helper.go"] = "package create\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "fora do trio") {
		t.Fatalf("esperava violação de arquivo fora do trio: %v", err)
	}
}

func TestValidateScaffoldLayout_Domain_ArquivoExtra(t *testing.T) {
	files := fullOK()
	files["internal/application/domain/foo/ports.go"] = "package foo\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/application/domain/foo/ports.go") {
		t.Fatalf("esperava violação de arquivo extra em domain: %v", err)
	}
}

func TestValidateScaffoldLayout_Repository_NomeDivergente(t *testing.T) {
	files := fullOK()
	// renomeia foo.go para repository.go (decisão 1)
	delete(files, "internal/repositories/foo/foo.go")
	files["internal/repositories/foo/repository.go"] = "package foo\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/repositories/foo/repository.go") {
		t.Fatalf("esperava violação de nome de repo divergente: %v", err)
	}
}

func TestValidateScaffoldLayout_Repository_SemDomain(t *testing.T) {
	files := fullOK()
	files["internal/repositories/orphanrepo/orphanrepo.go"] = "package orphanrepo\n" // sem domain correspondente
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "repositório sem domain associado") {
		t.Fatalf("esperava violação de repo sem domain: %v", err)
	}
}

func TestValidateScaffoldLayout_Repository_BaseRepositoryPermitido(t *testing.T) {
	files := fullOK()
	files["internal/repositories/base_repository/base_repository.go"] = "package base_repository\n"
	files["internal/repositories/base_repository/query_builder.go"] = "package base_repository\n"
	root := setupFakeRepo(t, files)
	if err := ValidateScaffoldLayout(root); err != nil {
		t.Fatalf("base_repository/ não deveria virar violação: %v", err)
	}
}

// ----- branches de violação ainda não cobertos: subdiretórios proibidos em
// routes/middleware/domain/repository e arquivos soltos em service/repository.
// Cada teste parte de fullOK (conforme) e injeta uma única violação. -----

func TestValidateScaffoldLayout_Routes_Subdiretorio(t *testing.T) {
	files := fullOK()
	files["cmd/http/routes/sub/x.go"] = "package sub\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "cmd/http/routes/sub") ||
		!strings.Contains(err.Error(), "subdiretório não permitido em routes/") {
		t.Fatalf("esperava violação de subdiretório em routes/: %v", err)
	}
}

func TestValidateScaffoldLayout_Middleware_Subdiretorio(t *testing.T) {
	files := fullOK()
	files["cmd/http/middlewares/sub/x.go"] = "package sub\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "cmd/http/middlewares/sub") ||
		!strings.Contains(err.Error(), "subdiretório não permitido em middlewares/") {
		t.Fatalf("esperava violação de subdiretório em middlewares/: %v", err)
	}
}

func TestValidateScaffoldLayout_Service_ArquivoSoltoNaRaiz(t *testing.T) {
	files := fullOK()
	files["internal/application/services/helper.go"] = "package services\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/application/services/helper.go") ||
		!strings.Contains(err.Error(), "arquivo solto na raiz de services/") {
		t.Fatalf("esperava violação de arquivo solto na raiz de services/: %v", err)
	}
}

func TestValidateScaffoldLayout_Service_ArquivoNoNivelDoDominio(t *testing.T) {
	files := fullOK()
	files["internal/application/services/foo/helper.go"] = "package foo\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/application/services/foo/helper.go") ||
		!strings.Contains(err.Error(), "service deve viver em services/<domain>/<verb>/") {
		t.Fatalf("esperava violação de arquivo no nível do domínio em services/: %v", err)
	}
}

func TestValidateScaffoldLayout_Service_SubpacoteNoVerbo(t *testing.T) {
	files := fullOK()
	files["internal/application/services/foo/create/inner/x.go"] = "package inner\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/application/services/foo/create/inner") ||
		!strings.Contains(err.Error(), "subpacote não permitido dentro do verbo") {
		t.Fatalf("esperava violação de subpacote no verbo de service: %v", err)
	}
}

func TestValidateScaffoldLayout_Domain_Subdiretorio(t *testing.T) {
	files := fullOK()
	files["internal/application/domain/foo/sub/x.go"] = "package sub\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/application/domain/foo/sub") ||
		!strings.Contains(err.Error(), "subdiretório não permitido em domain/") {
		t.Fatalf("esperava violação de subdiretório em domain/: %v", err)
	}
}

func TestValidateScaffoldLayout_Repository_ArquivoSoltoNaRaiz(t *testing.T) {
	files := fullOK()
	files["internal/repositories/helper.go"] = "package repositories\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/repositories/helper.go") ||
		!strings.Contains(err.Error(), "arquivo solto na raiz de repositories/") {
		t.Fatalf("esperava violação de arquivo solto na raiz de repositories/: %v", err)
	}
}

func TestValidateScaffoldLayout_Repository_Subdiretorio(t *testing.T) {
	files := fullOK()
	files["internal/repositories/foo/sub/x.go"] = "package sub\n"
	root := setupFakeRepo(t, files)
	err := ValidateScaffoldLayout(root)
	if err == nil || !strings.Contains(err.Error(), "internal/repositories/foo/sub") ||
		!strings.Contains(err.Error(), "subdiretório não permitido em repositories/") {
		t.Fatalf("esperava violação de subdiretório em repositories/<domain>/: %v", err)
	}
}
