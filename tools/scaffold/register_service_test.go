package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterService_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedBootstrapServices(t, root)

	rel, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err != nil {
		t.Fatalf("RegisterService: %v", err)
	}
	if want := filepath.Join("bootstrap", "services.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`"zord/internal/application/services/auth/login"`,
		"reg.Provide(login.RegistryKey, login.NewService(log, idC))",
	)
	mustParse(t, got)
}

func TestRegisterService_HappyPath_CompoundVerb(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "SelectOrg")
	seedBootstrapServices(t, root)

	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "SelectOrg"}); err != nil {
		t.Fatalf("RegisterService: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	mustContain(t, got,
		`"zord/internal/application/services/auth/select_org"`,
		"reg.Provide(select_org.RegistryKey, select_org.NewService(log, idC))",
	)
}

func TestRegisterService_FailsIfServiceMissing(t *testing.T) {
	root := t.TempDir()
	seedBootstrapServices(t, root)
	// service não criado

	_, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("RegisterService: esperado erro pra service ausente")
	}
	if !strings.Contains(err.Error(), "service") {
		t.Errorf("erro %q não menciona service", err.Error())
	}
}

func TestRegisterService_FailsIfImportAlreadyPresent(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedBootstrapServices(t, root)

	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("primeiro RegisterService: %v", err)
	}
	_, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("segundo RegisterService idêntico: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já presente") {
		t.Errorf("erro %q não menciona 'já presente'", err.Error())
	}
}

func TestRegisterService_AppliesAliasOnCollision(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Org")
	seedDomain(t, root, "Billing")
	seedService(t, root, "Org", "Create")
	seedService(t, root, "Billing", "Create")
	seedBootstrapServices(t, root)

	// Primeiro registro fica com import bare (`create`).
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Org", Verb: "Create"}); err != nil {
		t.Fatalf("primeiro RegisterService: %v", err)
	}
	// Segundo registro com mesmo verbo em outro domain deve aplicar alias.
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Billing", Verb: "Create"}); err != nil {
		t.Fatalf("segundo RegisterService: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	mustContain(t, got,
		`"zord/internal/application/services/org/create"`,
		`billing_create "zord/internal/application/services/billing/create"`,
		"reg.Provide(create.RegistryKey, create.NewService(log, idC))",
		"reg.Provide(billing_create.RegistryKey, billing_create.NewService(log, idC))",
	)
	mustParse(t, got)
}

func TestRegisterService_FailsOnInvalidIdent(t *testing.T) {
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
			_, err := RegisterService(RegisterServiceOptions{Domain: tc.domain, Verb: tc.verb})
			if err == nil {
				t.Fatalf("RegisterService(%q,%q): esperado erro, got nil", tc.domain, tc.verb)
			}
		})
	}
}

func TestRegisterService_DoesNotMutateOnFailure(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedBootstrapServices(t, root)

	// Registra com sucesso uma vez.
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("primeiro RegisterService: %v", err)
	}
	before := readFile(t, filepath.Join(root, "bootstrap", "services.go"))

	// Re-registra (deve falhar por idempotência) — arquivo não pode mudar.
	if _, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"}); err == nil {
		t.Fatalf("segundo RegisterService: esperado erro, got nil")
	}
	after := readFile(t, filepath.Join(root, "bootstrap", "services.go"))
	if before != after {
		t.Fatalf("arquivo mutado após falha:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestRegisterService_FailsIfBootstrapMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	// bootstrap/services.go ausente

	_, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("RegisterService: esperado erro pra bootstrap ausente")
	}
}

func TestRegisterService_FailsIfRegisterFuncMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	relFile := filepath.Join("bootstrap", "services.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// bootstrap/services.go sem a função registerServices
	src := `package bootstrap

import "zord/pkg/registry"

func registerOther(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil || !strings.Contains(err.Error(), registerServicesFunc) {
		t.Fatalf("RegisterService: esperado erro mencionando %s, got: %v", registerServicesFunc, err)
	}
}

func TestRegisterService_FailsIfProvideCallAlreadyPresent(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	// bootstrap/services.go pré-populado com a linha de Provide mas SEM o
	// import — caso patológico para garantir que a checagem de Provide funciona
	// independentemente do estado do bloco de imports.
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

	_, err := RegisterService(RegisterServiceOptions{Root: root, Domain: "Auth", Verb: "Login"})
	if err == nil {
		t.Fatalf("RegisterService: esperado erro pra Provide já presente, got nil")
	}
	if !strings.Contains(err.Error(), "já presente") {
		t.Errorf("erro %q não menciona 'já presente'", err.Error())
	}
}

// --- seeding helpers (compartilhados com testes futuros do pacote register) ---

// seedBootstrapServices grava um `bootstrap/services.go` mínimo: a função
// `registerServices(reg *registry.Registry)` com corpo vazio. É o esqueleto
// canônico que `service register` espera encontrar.
func seedBootstrapServices(t *testing.T, root string) {
	t.Helper()
	relFile := filepath.Join("bootstrap", "services.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerServices(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed bootstrap/services.go: %v", err)
	}
}

func mustParse(t *testing.T, src string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution); err != nil {
		t.Fatalf("arquivo patchado não compila no parser: %v\n%s", err, src)
	}
}
