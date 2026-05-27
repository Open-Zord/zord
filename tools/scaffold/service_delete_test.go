package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedServiceWithoutBootstrap seeda só o domain + service, sem bootstrap.
// Cobre o cenário em que `service delete` é o primeiro comando rodado num
// repo que ainda não tem wire-up.
func seedServiceWithoutBootstrap(t *testing.T, root, dom, verb string) {
	t.Helper()
	seedDomain(t, root, dom)
	seedService(t, root, dom, verb)
}

// seedHandlerDir cria só o diretório do handler 1:1 (sem arquivo). Suficiente
// pra disparar a guarda do delete, que verifica existência da pasta via
// os.Stat (não inspeciona conteúdo).
func seedHandlerDir(t *testing.T, root, dom, verb string) {
	t.Helper()
	dir := filepath.Join(root, handlersBasePath, ToSnake(dom), ToSnake(verb))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir handler dir: %v", err)
	}
}

func TestServiceDelete_HappyPath_BootstrapAbsent(t *testing.T) {
	root := t.TempDir()
	seedServiceWithoutBootstrap(t, root, "Auth", "Login")

	rel, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err != nil {
		t.Fatalf("ServiceDelete: %v", err)
	}
	want := filepath.Join(servicesBasePath, "auth", "login")
	if rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	if _, err := os.Stat(filepath.Join(root, want)); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete: err=%v", err)
	}
}

func TestServiceDelete_HappyPath_BootstrapPresentNoWireUp(t *testing.T) {
	root := t.TempDir()
	seedServiceWithoutBootstrap(t, root, "Auth", "Login")
	seedBootstrapServices(t, root)

	rel, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err != nil {
		t.Fatalf("ServiceDelete: %v", err)
	}
	if rel != filepath.Join(servicesBasePath, "auth", "login") {
		t.Errorf("path inesperado: %q", rel)
	}
	if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete")
	}
	// Bootstrap intacto.
	got := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	mustContain(t, got, "func registerServices(reg *registry.Registry)")
}

func TestServiceDelete_HappyPath_BootstrapSemRegisterFunc(t *testing.T) {
	root := t.TempDir()
	seedServiceWithoutBootstrap(t, root, "Auth", "Login")
	// Bootstrap presente mas sem `registerServices` — não há wire-up possível.
	absFile := filepath.Join(root, "bootstrap", "services.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerOther(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("ServiceDelete: %v", err)
	}
}

func TestServiceDelete_FailsIfServiceMissing(t *testing.T) {
	root := t.TempDir()
	// Pasta nunca foi criada.

	_, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra service ausente, got nil")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("erro %q não menciona 'não existe'", err.Error())
	}
}

func TestServiceDelete_FailsIfImportStillPresent(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedBootstrapServices(t, root)
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}

	before := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	_, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra wire-up vivo, got nil")
	}
	if !strings.Contains(err.Error(), "import") || !strings.Contains(err.Error(), "scaffold service unregister") {
		t.Errorf("erro %q não orienta a rodar unregister", err.Error())
	}
	// Pasta intocada.
	if _, statErr := os.Stat(filepath.Join(root, servicesBasePath, "auth", "login")); statErr != nil {
		t.Errorf("pasta foi apagada apesar do erro: %v", statErr)
	}
	// Bootstrap intocado.
	after := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	if before != after {
		t.Errorf("bootstrap mutado em falha:\n%s", after)
	}
}

func TestServiceDelete_FailsIfProvideStillPresentWithoutImport(t *testing.T) {
	// Caso patológico: arquivo com Provide mas sem o import equivalente.
	// service delete precisa rejeitar mesmo assim.
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	absFile := filepath.Join(root, "bootstrap", "services.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerServices(reg *registry.Registry) {
	reg.Provide(login.RegistryKey, login.NewService(log, idC))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra Provide vivo sem import, got nil")
	}
	if !strings.Contains(err.Error(), "Provide") {
		t.Errorf("erro %q não menciona Provide", err.Error())
	}
}

func TestServiceDelete_FailsIfAliasedProvideStillPresent(t *testing.T) {
	// Cenário: dois domains com mesmo verbo `Create` força alias no segundo.
	// Tenta apagar o segundo enquanto o alias ainda está em uso.
	root := t.TempDir()
	seedDomain(t, root, "Org")
	seedDomain(t, root, "Billing")
	seedService(t, root, "Org", "Create")
	seedService(t, root, "Billing", "Create")
	seedBootstrapServices(t, root)
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Org", Verb: "Create"}); err != nil {
		t.Fatalf("RegisterService Org: %v", err)
	}
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Billing", Verb: "Create"}); err != nil {
		t.Fatalf("RegisterService Billing: %v", err)
	}

	_, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Billing", Verb: "Create"})
	if err == nil {
		t.Fatalf("esperado erro pra alias vivo, got nil")
	}
	if !strings.Contains(err.Error(), "import") {
		t.Errorf("erro %q não menciona import (a guarda primária é o import)", err.Error())
	}
}

func TestServiceDelete_SucceedsWhenColidingBasenameImportStaysAlive(t *testing.T) {
	// Cenário do NAVE-105: dois services com mesmo basename de pacote
	// (`create`), um bare (Org.Create) e um aliased (Billing.Create →
	// billing_create). Após `service unregister Billing Create`, o wire-up
	// do Billing some mas o bare `reg.Provide(create.RegistryKey, ...)` do
	// Org continua vivo. `service delete Billing Create` deve PASSAR — o
	// `create` que sobrou pertence ao Org, não ao Billing.
	root := t.TempDir()
	seedDomain(t, root, "Org")
	seedDomain(t, root, "Billing")
	seedService(t, root, "Org", "Create")
	seedService(t, root, "Billing", "Create")
	seedBootstrapServices(t, root)
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Org", Verb: "Create"}); err != nil {
		t.Fatalf("RegisterService Org: %v", err)
	}
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Billing", Verb: "Create"}); err != nil {
		t.Fatalf("RegisterService Billing: %v", err)
	}
	if _, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Billing", Verb: "Create"}); err != nil {
		t.Fatalf("UnregisterService Billing: %v", err)
	}
	// Sanidade: bootstrap ainda tem o bare create do Org, e o alias do Billing
	// sumiu.
	bootstrap := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	mustContain(t, bootstrap,
		`"zord/internal/application/services/org/create"`,
		"reg.Provide(create.RegistryKey, create.NewService(log, idC))",
	)
	mustNotContain(t, bootstrap,
		`billing_create "zord/internal/application/services/billing/create"`,
		"billing_create.RegistryKey",
	)

	rel, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Billing", Verb: "Create"})
	if err != nil {
		t.Fatalf("ServiceDelete Billing Create: esperado sucesso (basename `create` pertence ao Org), got erro: %v", err)
	}
	wantRel := filepath.Join(servicesBasePath, "billing", "create")
	if rel != wantRel {
		t.Errorf("path: got %q, want %q", rel, wantRel)
	}
	// Pasta do Billing apagada.
	if _, statErr := os.Stat(filepath.Join(root, wantRel)); !os.IsNotExist(statErr) {
		t.Errorf("pasta billing/create ainda existe após delete: %v", statErr)
	}
	// Pasta do Org intocada.
	if _, statErr := os.Stat(filepath.Join(root, servicesBasePath, "org", "create")); statErr != nil {
		t.Errorf("pasta org/create foi tocada: %v", statErr)
	}
	// Bootstrap continua referenciando o Org.
	after := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	mustContain(t, after,
		`"zord/internal/application/services/org/create"`,
		"reg.Provide(create.RegistryKey, create.NewService(log, idC))",
	)
}

func TestServiceDelete_StillFailsWhenColidingBasenameAndOwnWireUpAlive(t *testing.T) {
	// Inverso do teste acima: a guarda AINDA precisa disparar quando o
	// wire-up do verbo a deletar continua vivo, mesmo havendo colisão de
	// basename com outro import. Aqui Billing.Create (aliased) tem import e
	// Provide vivos — service delete deve rejeitar pelo import.
	root := t.TempDir()
	seedDomain(t, root, "Org")
	seedDomain(t, root, "Billing")
	seedService(t, root, "Org", "Create")
	seedService(t, root, "Billing", "Create")
	seedBootstrapServices(t, root)
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Org", Verb: "Create"}); err != nil {
		t.Fatalf("RegisterService Org: %v", err)
	}
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Billing", Verb: "Create"}); err != nil {
		t.Fatalf("RegisterService Billing: %v", err)
	}

	before := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	_, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Billing", Verb: "Create"})
	if err == nil {
		t.Fatalf("esperado erro pra wire-up vivo do Billing (alias billing_create), got nil")
	}
	if !strings.Contains(err.Error(), "import") || !strings.Contains(err.Error(), "scaffold service unregister") {
		t.Errorf("erro %q não orienta a rodar unregister", err.Error())
	}
	// Pasta intocada.
	if _, statErr := os.Stat(filepath.Join(root, servicesBasePath, "billing", "create")); statErr != nil {
		t.Errorf("pasta foi apagada apesar do erro: %v", statErr)
	}
	// Bootstrap intocado.
	after := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	if before != after {
		t.Errorf("bootstrap mutado em falha")
	}
}

func TestServiceDelete_FailsIfHandlerStillPresent(t *testing.T) {
	root := t.TempDir()
	seedServiceWithoutBootstrap(t, root, "Auth", "Login")
	seedHandlerDir(t, root, "Auth", "Login")

	_, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra handler vivo, got nil")
	}
	if !strings.Contains(err.Error(), "handler") {
		t.Errorf("erro %q não menciona handler", err.Error())
	}
	// Pasta do service intocada.
	if _, statErr := os.Stat(filepath.Join(root, servicesBasePath, "auth", "login")); statErr != nil {
		t.Errorf("pasta do service foi apagada apesar do erro: %v", statErr)
	}
}

func TestServiceDelete_FailsOnInvalidIdent(t *testing.T) {
	cases := []struct {
		name   string
		domain string
		verb   string
	}{
		{"domain lowercase", "auth", "Login"},
		{"domain empty", "", "Login"},
		{"domain non-ident", "Auth-Org", "Login"},
		{"verb lowercase", "Auth", "login"},
		{"verb empty", "Auth", ""},
		{"verb non-ident", "Auth", "Login-Now"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ServiceDelete(ServiceDeleteOptions{Domain: tc.domain, Verb: tc.verb})
			if err == nil {
				t.Fatalf("ServiceDelete(%q,%q): esperado erro, got nil", tc.domain, tc.verb)
			}
		})
	}
}

func TestServiceDelete_IsIdempotentFailureAfterSuccess(t *testing.T) {
	root := t.TempDir()
	seedServiceWithoutBootstrap(t, root, "Auth", "Login")

	if _, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("primeiro ServiceDelete: %v", err)
	}
	_, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("segundo ServiceDelete: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("erro idempotente %q não menciona 'não existe'", err.Error())
	}
}

func TestServiceDelete_FullCycleAfterUnregister(t *testing.T) {
	// Fluxo end-to-end: create → register → unregister → delete deve passar.
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedBootstrapServices(t, root)
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}
	if _, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("UnregisterService: %v", err)
	}

	rel, err := ServiceDelete(ServiceDeleteOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err != nil {
		t.Fatalf("ServiceDelete após unregister: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(statErr) {
		t.Errorf("pasta ainda existe: %v", statErr)
	}
}
