package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// registerForUnregister é um atalho pra preparar o estado pré-unregister:
// seed do domínio, do service e do bootstrap/services.go, seguido do
// RegisterService de verdade. Falha o teste se qualquer passo falha.
func registerForUnregister(t *testing.T, root, domain, verb string) {
	t.Helper()
	seedDomain(t, root, domain)
	seedService(t, root, domain, verb)
	seedBootstrapServices(t, root)
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: domain, Verb: verb}); err != nil {
		t.Fatalf("RegisterService(%s,%s): %v", domain, verb, err)
	}
}

func TestUnregisterService_HappyPath_BareImport(t *testing.T) {
	root := t.TempDir()
	registerForUnregister(t, root, "Auth", "Login")
	// Sanity: o registro deve ter sido bare.
	before := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	if !strings.Contains(before, `"zord/internal/application/services/auth/login"`) {
		t.Fatalf("pré-condição falhou: import bare ausente:\n%s", before)
	}

	rel, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err != nil {
		t.Fatalf("UnregisterService: %v", err)
	}
	if want := filepath.Join("bootstrap", "services.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustNotContain(t, got,
		`"zord/internal/application/services/auth/login"`,
		"login.RegistryKey",
		"login.NewService",
	)
	mustParse(t, got)
}

func TestUnregisterService_HappyPath_AliasedImport(t *testing.T) {
	root := t.TempDir()
	// Setup: dois domains com mesmo verbo `Create` força alias no segundo.
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
	if !strings.Contains(before, `billing_create "zord/internal/application/services/billing/create"`) {
		t.Fatalf("pré-condição falhou: alias billing_create ausente:\n%s", before)
	}

	if _, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Billing", Verb: "Create"}); err != nil {
		t.Fatalf("UnregisterService Billing: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	// Remoção do aliased não pode afetar o registro bare do Org.Create.
	mustNotContain(t, got,
		`billing_create "zord/internal/application/services/billing/create"`,
		"billing_create.RegistryKey",
	)
	mustContain(t, got,
		`"zord/internal/application/services/org/create"`,
		"reg.Provide(create.RegistryKey, create.NewService(log, idC))",
	)
	mustParse(t, got)
}

func TestUnregisterService_FailsIfImportMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/services.go pré-populado com a linha de Provide mas SEM o
	// import — caso patológico simétrico ao do register: garante que a
	// validação do import roda antes da remoção do Provide.
	relFile := filepath.Join("bootstrap", "services.go")
	absFile := filepath.Join(root, relFile)
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

	_, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("UnregisterService: esperado erro pra import ausente, got nil")
	}
	if !strings.Contains(err.Error(), "import") || !strings.Contains(err.Error(), "ausente") {
		t.Errorf("erro %q não menciona 'import ... ausente'", err.Error())
	}
	after := readFile(t, absFile)
	if after != src {
		t.Fatalf("arquivo mutado em falha:\n--- antes ---\n%s\n--- depois ---\n%s", src, after)
	}
}

func TestUnregisterService_FailsIfProvideMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/services.go com o import presente mas sem a linha de Provide.
	relFile := filepath.Join("bootstrap", "services.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import (
	"zord/internal/application/services/auth/login"
	"zord/pkg/registry"
)

var _ = login.RegistryKey // mantém o import em uso pro arquivo compilar

func registerServices(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("UnregisterService: esperado erro pra Provide ausente, got nil")
	}
	if !strings.Contains(err.Error(), "Provide") || !strings.Contains(err.Error(), "ausente") {
		t.Errorf("erro %q não menciona 'Provide ... ausente'", err.Error())
	}
	after := readFile(t, absFile)
	if after != src {
		t.Fatalf("arquivo mutado em falha:\n--- antes ---\n%s\n--- depois ---\n%s", src, after)
	}
}

func TestUnregisterService_FailsIfBothMissing(t *testing.T) {
	root := t.TempDir()
	seedBootstrapServices(t, root)
	// Nem import, nem linha de Provide.

	_, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("UnregisterService: esperado erro, got nil")
	}
	// O primeiro check é o do import — a mensagem deve refletir isso.
	if !strings.Contains(err.Error(), "import") {
		t.Errorf("erro %q não menciona import (primeira validação)", err.Error())
	}
}

func TestUnregisterService_FailsIfBootstrapMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/services.go ausente.

	_, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("UnregisterService: esperado erro pra bootstrap ausente")
	}
}

func TestUnregisterService_FailsIfRegisterFuncMissing(t *testing.T) {
	root := t.TempDir()
	relFile := filepath.Join("bootstrap", "services.go")
	absFile := filepath.Join(root, relFile)
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

	_, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil || !strings.Contains(err.Error(), registerServicesFunc) {
		t.Fatalf("UnregisterService: esperado erro mencionando %s, got: %v", registerServicesFunc, err)
	}
}

func TestUnregisterService_FailsOnInvalidIdent(t *testing.T) {
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
			_, err := UnregisterService(UnregisterServiceOptions{Domain: tc.domain, Verb: tc.verb})
			if err == nil {
				t.Fatalf("UnregisterService(%q,%q): esperado erro, got nil", tc.domain, tc.verb)
			}
		})
	}
}

func TestUnregisterService_IsIdempotentFailureAfterSuccess(t *testing.T) {
	root := t.TempDir()
	registerForUnregister(t, root, "Auth", "Login")

	if _, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("primeiro UnregisterService: %v", err)
	}
	afterFirst := readFile(t, filepath.Join(root, "bootstrap", "services.go"))

	// Segundo unregister sobre o mesmo par já desligado — deve falhar sem mutar.
	if _, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err == nil {
		t.Fatalf("segundo UnregisterService: esperado erro, got nil")
	}
	afterSecond := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	if afterFirst != afterSecond {
		t.Fatalf("arquivo mutado em falha idempotente:\n--- depois do 1º ---\n%s\n--- depois do 2º ---\n%s", afterFirst, afterSecond)
	}
}

// TestUnregisterService_RoundTripIsByteIdentical garante que register seguido
// de unregister devolve o arquivo byte-a-byte ao estado original. Trava
// regressão do bug onde o printer deixava linha em branco residual entre o
// último stmt e o Rbrace após remoção do ExprStmt.
//
// Usa seed com import já em forma grouped e um Provide pré-existente — bate
// com o shape real de bootstrap/services.go. Seed bare (1 único import) +
// função vazia escapa do invariante porque astutil.AddNamedImport converte
// bare em grouped no primeiro insert, e essa transformação não é revertida no
// remove (comportamento esperado do astutil).
func TestUnregisterService_RoundTripIsByteIdentical(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	relFile := filepath.Join("bootstrap", "services.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import (
	"zord/internal/application/services"
	"zord/internal/application/services/session/logout"
	"zord/pkg/idCreator"
	"zord/pkg/logger"
	"zord/pkg/registry"
)

func registerServices(reg *registry.Registry) {
	log := registry.Resolve[services.Logger](reg, logger.RegistryKey)
	idC := registry.Resolve[services.IdCreator](reg, idCreator.RegistryKey)
	reg.Provide(logout.RegistryKey, logout.NewService(log, idC))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}
	if _, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("UnregisterService: %v", err)
	}

	after := readFile(t, absFile)
	if src != after {
		t.Fatalf("round-trip não é byte-idêntico:\n--- before ---\n%s\n--- after ---\n%s", src, after)
	}
}

func TestUnregisterService_PreservesSiblingProvides(t *testing.T) {
	root := t.TempDir()
	// Registra dois services no mesmo domain; unregister só do segundo.
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedService(t, root, "Auth", "Logout")
	seedBootstrapServices(t, root)
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("RegisterService Login: %v", err)
	}
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Logout"}); err != nil {
		t.Fatalf("RegisterService Logout: %v", err)
	}

	if _, err := UnregisterService(UnregisterServiceOptions{Root: root, Domain: "Auth", Verb: "Logout"}); err != nil {
		t.Fatalf("UnregisterService Logout: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	// Login intacto.
	mustContain(t, got,
		`"zord/internal/application/services/auth/login"`,
		"reg.Provide(login.RegistryKey, login.NewService(log, idC))",
	)
	// Logout removido.
	mustNotContain(t, got,
		`"zord/internal/application/services/auth/logout"`,
		"logout.RegistryKey",
		"logout.NewService",
	)
	mustParse(t, got)
}
