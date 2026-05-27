package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterHandler_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)

	rel, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}
	if want := filepath.Join("bootstrap", "handlers.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`authloginhandler "zord/cmd/http/handlers/auth/login"`,
		"reg.Provide(authloginhandler.RegistryKey, authloginhandler.NewLoginHandler(reg))",
	)
	mustParse(t, got)
}

func TestRegisterHandler_HappyPath_CompoundDomain(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "UsageRecord")
	seedService(t, root, "UsageRecord", "Export")
	seedHandler(t, root, "UsageRecord", "Export")
	seedBootstrapHandlers(t, root)

	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "UsageRecord", Service: "Export"}); err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	mustContain(t, got,
		`usagerecordexporthandler "zord/cmd/http/handlers/usage_record/export"`,
		"reg.Provide(usagerecordexporthandler.RegistryKey, usagerecordexporthandler.NewExportHandler(reg))",
	)
	mustParse(t, got)
}

func TestRegisterHandler_HappyPath_CompoundService(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Org")
	seedService(t, root, "Org", "CreateMembership")
	seedHandler(t, root, "Org", "CreateMembership")
	seedBootstrapHandlers(t, root)

	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Org", Service: "CreateMembership"}); err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	mustContain(t, got,
		`orgcreatemembershiphandler "zord/cmd/http/handlers/org/create_membership"`,
		"reg.Provide(orgcreatemembershiphandler.RegistryKey, orgcreatemembershiphandler.NewCreateMembershipHandler(reg))",
	)
	mustParse(t, got)
}

func TestRegisterHandler_FailsIfHandlerFileMissing(t *testing.T) {
	root := t.TempDir()
	seedBootstrapHandlers(t, root)
	// arquivo do handler não criado

	_, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("RegisterHandler: esperado erro pra handler ausente")
	}
	if !strings.Contains(err.Error(), "handler") {
		t.Errorf("erro %q não menciona handler", err.Error())
	}
}

func TestRegisterHandler_FailsIfRegistryKeyMissing(t *testing.T) {
	root := t.TempDir()
	seedHandlerFileWithoutRegistryKey(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)

	_, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil || !strings.Contains(err.Error(), "RegistryKey") {
		t.Fatalf("RegisterHandler: esperado erro mencionando RegistryKey, got: %v", err)
	}
}

func TestRegisterHandler_FailsIfConstructorMissing(t *testing.T) {
	root := t.TempDir()
	seedHandlerFileWithoutConstructor(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)

	_, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil || !strings.Contains(err.Error(), "NewLoginHandler") {
		t.Fatalf("RegisterHandler: esperado erro mencionando NewLoginHandler, got: %v", err)
	}
}

func TestRegisterHandler_FailsIfImportAlreadyPresent(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)

	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("primeiro RegisterHandler: %v", err)
	}
	_, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("segundo RegisterHandler idêntico: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já presente") {
		t.Errorf("erro %q não menciona 'já presente'", err.Error())
	}
}

func TestRegisterHandler_FailsIfProvideCallAlreadyPresent(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	// bootstrap/handlers.go pré-populado com a linha de Provide mas SEM o
	// import — caso patológico pra garantir que a checagem de Provide funciona
	// independentemente do estado do bloco de imports.
	relFile := filepath.Join("bootstrap", "handlers.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerHandlers(reg *registry.Registry) {
	reg.Provide(authloginhandler.RegistryKey, authloginhandler.NewLoginHandler(reg))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("RegisterHandler: esperado erro pra Provide já presente, got nil")
	}
	if !strings.Contains(err.Error(), "já presente") {
		t.Errorf("erro %q não menciona 'já presente'", err.Error())
	}
}

func TestRegisterHandler_FailsOnInvalidDomain(t *testing.T) {
	cases := []struct {
		name   string
		domain string
	}{
		{"lowercase", "auth"},
		{"empty", ""},
		{"non-ident", "Auth-Service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RegisterHandler(RegisterHandlerOptions{Domain: tc.domain, Service: "Login"})
			if err == nil {
				t.Fatalf("RegisterHandler(domain=%q): esperado erro, got nil", tc.domain)
			}
		})
	}
}

func TestRegisterHandler_FailsOnInvalidService(t *testing.T) {
	cases := []struct {
		name    string
		service string
	}{
		{"lowercase", "login"},
		{"empty", ""},
		{"non-ident", "Log-In"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RegisterHandler(RegisterHandlerOptions{Domain: "Auth", Service: tc.service})
			if err == nil {
				t.Fatalf("RegisterHandler(service=%q): esperado erro, got nil", tc.service)
			}
		})
	}
}

func TestRegisterHandler_FailsIfBootstrapMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	// bootstrap/handlers.go ausente

	_, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("RegisterHandler: esperado erro pra bootstrap ausente")
	}
}

func TestRegisterHandler_FailsIfRegisterFuncMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	relFile := filepath.Join("bootstrap", "handlers.go")
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

	_, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil || !strings.Contains(err.Error(), registerHandlersFunc) {
		t.Fatalf("RegisterHandler: esperado erro mencionando %s, got: %v", registerHandlersFunc, err)
	}
}

func TestRegisterHandler_DoesNotMutateOnFailure(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)

	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("primeiro RegisterHandler: %v", err)
	}
	before := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))

	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err == nil {
		t.Fatalf("segundo RegisterHandler: esperado erro, got nil")
	}
	after := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	if before != after {
		t.Fatalf("arquivo mutado após falha:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

// --- seeding helpers ---

// seedHandler grava o arquivo do handler 1:1 usando o próprio scaffold da
// NAVE-70. Garante consistência com o output real (`const RegistryKey`,
// `func New<Pascal>RegisterHandler`).
func seedHandler(t *testing.T, root, dom, service string) {
	t.Helper()
	if _, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: dom, Service: service}); err != nil {
		t.Fatalf("seed handler %s/%s: %v", dom, service, err)
	}
}

// seedHandlerFileWithoutRegistryKey grava um handler mínimo sem `const RegistryKey`
// pra exercer a checagem de assertHandlerExists. Não usa o scaffold da NAVE-70
// (que sempre emite RegistryKey).
func seedHandlerFileWithoutRegistryKey(t *testing.T, root, dom, service string) {
	t.Helper()
	snakeDomain := ToSnake(dom)
	snakeService := ToSnake(service)
	absDir := filepath.Join(root, "cmd/http/handlers", snakeDomain, snakeService)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fmt.Sprintf(`package %s

import "zord/pkg/registry"

type %sHandler struct {
	reg *registry.Registry
}

func New%sHandler(reg *registry.Registry) *%sHandler {
	return &%sHandler{reg: reg}
}
`, snakeService, service, service, service, service)
	absFile := filepath.Join(absDir, "handler.go")
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed handler: %v", err)
	}
}

// seedHandlerFileWithoutConstructor grava um handler mínimo sem o constructor
// `New<Pascal>RegisterHandler` pra exercer a checagem de assertHandlerExists.
func seedHandlerFileWithoutConstructor(t *testing.T, root, dom, service string) {
	t.Helper()
	snakeDomain := ToSnake(dom)
	snakeService := ToSnake(service)
	absDir := filepath.Join(root, "cmd/http/handlers", snakeDomain, snakeService)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fmt.Sprintf(`package %s

import "zord/pkg/registry"

const RegistryKey = %q

type %sHandler struct {
	reg *registry.Registry
}
`, snakeService, ToLowerCamel(service)+"RegisterHandler", service)
	absFile := filepath.Join(absDir, "handler.go")
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed handler: %v", err)
	}
}

// seedBootstrapHandlers grava um `bootstrap/handlers.go` mínimo: a função
// `registerHandlers(reg *registry.Registry)` com corpo vazio.
func seedBootstrapHandlers(t *testing.T, root string) {
	t.Helper()
	relFile := filepath.Join("bootstrap", "handlers.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerHandlers(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed bootstrap/handlers.go: %v", err)
	}
}
