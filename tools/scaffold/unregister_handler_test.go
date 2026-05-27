package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// registerHandlerForUnregister é um atalho pra preparar o estado pré-unregister:
// seed do domínio, do service, do handler e do bootstrap/handlers.go, seguido
// do RegisterHandler de verdade. Falha o teste se qualquer passo falha.
func registerHandlerForUnregister(t *testing.T, root, domain, service string) {
	t.Helper()
	seedDomain(t, root, domain)
	seedService(t, root, domain, service)
	seedHandler(t, root, domain, service)
	seedBootstrapHandlers(t, root)
	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: domain, Service: service}); err != nil {
		t.Fatalf("RegisterHandler(%s,%s): %v", domain, service, err)
	}
}

func TestUnregisterHandler_HappyPath(t *testing.T) {
	root := t.TempDir()
	registerHandlerForUnregister(t, root, "Auth", "Login")
	before := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	if !strings.Contains(before, `authloginhandler "zord/cmd/http/handlers/auth/login"`) {
		t.Fatalf("pré-condição falhou: import aliased ausente:\n%s", before)
	}

	rel, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("UnregisterHandler: %v", err)
	}
	if want := filepath.Join("bootstrap", "handlers.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustNotContain(t, got,
		`authloginhandler "zord/cmd/http/handlers/auth/login"`,
		"authloginhandler.RegistryKey",
		"authloginhandler.NewLoginHandler",
	)
	mustParse(t, got)
}

func TestUnregisterHandler_HappyPath_CompoundDomain(t *testing.T) {
	root := t.TempDir()
	registerHandlerForUnregister(t, root, "UsageRecord", "Export")

	if _, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "UsageRecord", Service: "Export"}); err != nil {
		t.Fatalf("UnregisterHandler: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	mustNotContain(t, got,
		`usagerecordexporthandler "zord/cmd/http/handlers/usage_record/export"`,
		"usagerecordexporthandler.RegistryKey",
		"usagerecordexporthandler.NewExportHandler",
	)
	mustParse(t, got)
}

func TestUnregisterHandler_HappyPath_CompoundService(t *testing.T) {
	root := t.TempDir()
	registerHandlerForUnregister(t, root, "Org", "CreateMembership")

	if _, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Org", Service: "CreateMembership"}); err != nil {
		t.Fatalf("UnregisterHandler: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	mustNotContain(t, got,
		`orgcreatemembershiphandler "zord/cmd/http/handlers/org/create_membership"`,
		"orgcreatemembershiphandler.RegistryKey",
		"orgcreatemembershiphandler.NewCreateMembershipHandler",
	)
	mustParse(t, got)
}

func TestUnregisterHandler_FailsIfImportMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/handlers.go com linha de Provide mas SEM import — caso patológico
	// pra garantir que a validação do import roda antes da remoção do Provide.
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

	_, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("UnregisterHandler: esperado erro pra import ausente, got nil")
	}
	if !strings.Contains(err.Error(), "import") || !strings.Contains(err.Error(), "ausente") {
		t.Errorf("erro %q não menciona 'import ... ausente'", err.Error())
	}
	after := readFile(t, absFile)
	if after != src {
		t.Fatalf("arquivo mutado em falha:\n--- antes ---\n%s\n--- depois ---\n%s", src, after)
	}
}

func TestUnregisterHandler_FailsIfProvideMissing(t *testing.T) {
	root := t.TempDir()
	relFile := filepath.Join("bootstrap", "handlers.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import (
	authloginhandler "zord/cmd/http/handlers/auth/login"
	"zord/pkg/registry"
)

var _ = authloginhandler.RegistryKey // mantém o import em uso pro arquivo compilar

func registerHandlers(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("UnregisterHandler: esperado erro pra Provide ausente, got nil")
	}
	if !strings.Contains(err.Error(), "Provide") || !strings.Contains(err.Error(), "ausente") {
		t.Errorf("erro %q não menciona 'Provide ... ausente'", err.Error())
	}
	after := readFile(t, absFile)
	if after != src {
		t.Fatalf("arquivo mutado em falha:\n--- antes ---\n%s\n--- depois ---\n%s", src, after)
	}
}

func TestUnregisterHandler_FailsIfBothMissing(t *testing.T) {
	root := t.TempDir()
	seedBootstrapHandlers(t, root)

	_, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("UnregisterHandler: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "import") {
		t.Errorf("erro %q não menciona import (primeira validação)", err.Error())
	}
}

func TestUnregisterHandler_FailsIfBootstrapMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/handlers.go ausente.

	_, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("UnregisterHandler: esperado erro pra bootstrap ausente")
	}
}

func TestUnregisterHandler_FailsIfRegisterFuncMissing(t *testing.T) {
	root := t.TempDir()
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

	_, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil || !strings.Contains(err.Error(), registerHandlersFunc) {
		t.Fatalf("UnregisterHandler: esperado erro mencionando %s, got: %v", registerHandlersFunc, err)
	}
}

func TestUnregisterHandler_FailsOnInvalidIdent(t *testing.T) {
	cases := []struct {
		name    string
		domain  string
		service string
	}{
		{"domain lowercase", "auth", "Login"},
		{"domain empty", "", "Login"},
		{"domain non-ident", "Auth-Org", "Login"},
		{"service lowercase", "Auth", "login"},
		{"service empty", "Auth", ""},
		{"service non-ident", "Auth", "Login-Now"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := UnregisterHandler(UnregisterHandlerOptions{Domain: tc.domain, Service: tc.service})
			if err == nil {
				t.Fatalf("UnregisterHandler(%q,%q): esperado erro, got nil", tc.domain, tc.service)
			}
		})
	}
}

func TestUnregisterHandler_IsIdempotentFailureAfterSuccess(t *testing.T) {
	root := t.TempDir()
	registerHandlerForUnregister(t, root, "Auth", "Login")

	if _, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("primeiro UnregisterHandler: %v", err)
	}
	afterFirst := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))

	if _, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err == nil {
		t.Fatalf("segundo UnregisterHandler: esperado erro, got nil")
	}
	afterSecond := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	if afterFirst != afterSecond {
		t.Fatalf("arquivo mutado em falha idempotente:\n--- depois do 1º ---\n%s\n--- depois do 2º ---\n%s", afterFirst, afterSecond)
	}
}

// TestUnregisterHandler_RoundTripIsByteIdentical garante que register seguido
// de unregister devolve o arquivo byte-a-byte ao estado original. Trava
// regressão do bug do printer (linha em branco residual entre o último stmt e
// o Rbrace após remoção do ExprStmt) — lição da NAVE-88.
//
// Seed com import já em forma grouped e Provide pré-existente: bate com o
// shape real de bootstrap/handlers.go.
func TestUnregisterHandler_RoundTripIsByteIdentical(t *testing.T) {
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

import (
	authlogouthandler "zord/cmd/http/handlers/auth/logout"
	"zord/pkg/registry"
)

func registerHandlers(reg *registry.Registry) {
	reg.Provide(authlogouthandler.RegistryKey, authlogouthandler.NewLogoutHandler(reg))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}
	if _, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("UnregisterHandler: %v", err)
	}

	after := readFile(t, absFile)
	if src != after {
		t.Fatalf("round-trip não é byte-idêntico:\n--- before ---\n%s\n--- after ---\n%s", src, after)
	}
}

func TestUnregisterHandler_PreservesSiblingProvides(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedService(t, root, "Auth", "Logout")
	seedHandler(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Logout")
	seedBootstrapHandlers(t, root)
	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("RegisterHandler Login: %v", err)
	}
	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Logout"}); err != nil {
		t.Fatalf("RegisterHandler Logout: %v", err)
	}

	if _, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Logout"}); err != nil {
		t.Fatalf("UnregisterHandler Logout: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	mustContain(t, got,
		`authloginhandler "zord/cmd/http/handlers/auth/login"`,
		"reg.Provide(authloginhandler.RegistryKey, authloginhandler.NewLoginHandler(reg))",
	)
	mustNotContain(t, got,
		`authlogouthandler "zord/cmd/http/handlers/auth/logout"`,
		"authlogouthandler.RegistryKey",
		"authlogouthandler.NewLogoutHandler",
	)
	mustParse(t, got)
}
