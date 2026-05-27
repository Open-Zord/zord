package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// registerForUnregisterRepository é o atalho pra preparar o estado pré-unregister:
// seed do repository + bootstrap, seguido do RegisterRepository real. Falha o
// teste se qualquer passo falha.
func registerForUnregisterRepository(t *testing.T, root, domain string) {
	t.Helper()
	seedRepositoryFile(t, root, domain)
	seedBootstrapRepositories(t, root)
	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: domain}); err != nil {
		t.Fatalf("RegisterRepository(%s): %v", domain, err)
	}
}

func TestUnregisterRepository_HappyPath(t *testing.T) {
	root := t.TempDir()
	registerForUnregisterRepository(t, root, "Organization")
	before := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))
	if !strings.Contains(before, `organizationrepo "zord/internal/repositories/organization"`) {
		t.Fatalf("pré-condição falhou: import organizationrepo ausente:\n%s", before)
	}

	rel, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err != nil {
		t.Fatalf("UnregisterRepository: %v", err)
	}
	if want := filepath.Join("bootstrap", "repositories.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustNotContain(t, got,
		`organizationrepo "zord/internal/repositories/organization"`,
		"organizationrepo.RegistryKey",
		"organizationrepo.NewOrganizationRepository",
	)
	mustParse(t, got)
}

func TestUnregisterRepository_HappyPath_CompoundDomain(t *testing.T) {
	root := t.TempDir()
	registerForUnregisterRepository(t, root, "OrgMembership")

	if _, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "OrgMembership"}); err != nil {
		t.Fatalf("UnregisterRepository: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))
	mustNotContain(t, got,
		`orgmembershiprepo "zord/internal/repositories/org_membership"`,
		"orgmembershiprepo.RegistryKey",
		"orgmembershiprepo.NewOrgMembershipRepository",
	)
	mustParse(t, got)
}

func TestUnregisterRepository_FailsIfImportMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/repositories.go pré-populado com a linha de Provide mas SEM o
	// import — caso patológico simétrico ao do register: garante que a
	// validação do import roda antes da remoção do Provide.
	relFile := filepath.Join("bootstrap", "repositories.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerRepositories(reg *registry.Registry) {
	reg.Provide(organizationrepo.RegistryKey, organizationrepo.NewOrganizationRepository(db))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("UnregisterRepository: esperado erro pra import ausente, got nil")
	}
	if !strings.Contains(err.Error(), "import") || !strings.Contains(err.Error(), "ausente") {
		t.Errorf("erro %q não menciona 'import ... ausente'", err.Error())
	}
	after := readFile(t, absFile)
	if after != src {
		t.Fatalf("arquivo mutado em falha:\n--- antes ---\n%s\n--- depois ---\n%s", src, after)
	}
}

func TestUnregisterRepository_FailsIfProvideMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/repositories.go com o import presente mas sem a linha de Provide.
	relFile := filepath.Join("bootstrap", "repositories.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import (
	organizationrepo "zord/internal/repositories/organization"
	"zord/pkg/registry"
)

var _ = organizationrepo.RegistryKey // mantém o import em uso pro arquivo compilar

func registerRepositories(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("UnregisterRepository: esperado erro pra Provide ausente, got nil")
	}
	if !strings.Contains(err.Error(), "Provide") || !strings.Contains(err.Error(), "ausente") {
		t.Errorf("erro %q não menciona 'Provide ... ausente'", err.Error())
	}
	after := readFile(t, absFile)
	if after != src {
		t.Fatalf("arquivo mutado em falha:\n--- antes ---\n%s\n--- depois ---\n%s", src, after)
	}
}

func TestUnregisterRepository_FailsIfBothMissing(t *testing.T) {
	root := t.TempDir()
	seedBootstrapRepositories(t, root)
	// Nem import, nem linha de Provide.

	_, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("UnregisterRepository: esperado erro, got nil")
	}
	// Primeiro check é o do import.
	if !strings.Contains(err.Error(), "import") {
		t.Errorf("erro %q não menciona import (primeira validação)", err.Error())
	}
}

func TestUnregisterRepository_FailsIfBootstrapMissing(t *testing.T) {
	root := t.TempDir()
	// bootstrap/repositories.go ausente.

	_, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("UnregisterRepository: esperado erro pra bootstrap ausente")
	}
}

func TestUnregisterRepository_FailsIfRegisterFuncMissing(t *testing.T) {
	root := t.TempDir()
	relFile := filepath.Join("bootstrap", "repositories.go")
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

	_, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil || !strings.Contains(err.Error(), registerRepositoriesFunc) {
		t.Fatalf("UnregisterRepository: esperado erro mencionando %s, got: %v", registerRepositoriesFunc, err)
	}
}

func TestUnregisterRepository_FailsOnInvalidIdent(t *testing.T) {
	cases := []struct {
		name   string
		domain string
	}{
		{"lowercase", "organization"},
		{"empty", ""},
		{"non-ident", "Org-Membership"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := UnregisterRepository(UnregisterRepositoryOptions{Domain: tc.domain})
			if err == nil {
				t.Fatalf("UnregisterRepository(%q): esperado erro, got nil", tc.domain)
			}
		})
	}
}

func TestUnregisterRepository_IsIdempotentFailureAfterSuccess(t *testing.T) {
	root := t.TempDir()
	registerForUnregisterRepository(t, root, "Organization")

	if _, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("primeiro UnregisterRepository: %v", err)
	}
	afterFirst := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))

	if _, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"}); err == nil {
		t.Fatalf("segundo UnregisterRepository: esperado erro, got nil")
	}
	afterSecond := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))
	if afterFirst != afterSecond {
		t.Fatalf("arquivo mutado em falha idempotente:\n--- depois do 1º ---\n%s\n--- depois do 2º ---\n%s", afterFirst, afterSecond)
	}
}

func TestUnregisterRepository_PreservesSiblingProvides(t *testing.T) {
	root := t.TempDir()
	// Registra dois repositories; unregister só do segundo.
	seedRepositoryFile(t, root, "Organization")
	seedRepositoryFile(t, root, "PlatformUser")
	seedBootstrapRepositories(t, root)
	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("RegisterRepository Organization: %v", err)
	}
	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "PlatformUser"}); err != nil {
		t.Fatalf("RegisterRepository PlatformUser: %v", err)
	}

	if _, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "PlatformUser"}); err != nil {
		t.Fatalf("UnregisterRepository PlatformUser: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))
	// Organization intacto.
	mustContain(t, got,
		`organizationrepo "zord/internal/repositories/organization"`,
		"reg.Provide(organizationrepo.RegistryKey, organizationrepo.NewOrganizationRepository(db))",
	)
	// PlatformUser removido.
	mustNotContain(t, got,
		`platformuserrepo "zord/internal/repositories/platform_user"`,
		"platformuserrepo.RegistryKey",
		"platformuserrepo.NewPlatformUserRepository",
	)
	mustParse(t, got)
}

// TestUnregisterRepository_RoundTripIsByteIdentical garante que register
// seguido de unregister devolve o arquivo byte-a-byte ao estado original.
// Trava regressão do mesmo bug coberto em NAVE-88: o printer deixava linha em
// branco residual entre o último stmt e o Rbrace após remoção do ExprStmt.
//
// Seed usa import já em forma grouped e um Provide pré-existente — bate com o
// shape real de bootstrap/repositories.go. Seed bare (1 único import) + função
// vazia escapa do invariante porque astutil.AddNamedImport converte bare em
// grouped no primeiro insert, e essa transformação não é revertida no remove.
func TestUnregisterRepository_RoundTripIsByteIdentical(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	relFile := filepath.Join("bootstrap", "repositories.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import (
	userrepo "zord/internal/repositories/platform_user"
	"zord/pkg/registry"
)

func registerRepositories(reg *registry.Registry) {
	reg.Provide(userrepo.RegistryKey, userrepo.NewPlatformUserRepository(db))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("RegisterRepository: %v", err)
	}
	if _, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("UnregisterRepository: %v", err)
	}

	after := readFile(t, absFile)
	if src != after {
		t.Fatalf("round-trip não é byte-idêntico:\n--- before ---\n%s\n--- after ---\n%s", src, after)
	}
}

// TestUnregisterRepository_WorksWithShortenedAlias garante que o unregister
// funciona contra arquivos legados onde o dev encurtou o alias à mão (ex.:
// `orgrepo` em vez do `organizationrepo` default que NAVE-72 gera). O
// `bootstrap/repositories.go` real do the target repo usa essas formas curtas,
// então sem essa flexibilidade o comando não rodaria contra o codebase.
func TestUnregisterRepository_WorksWithShortenedAlias(t *testing.T) {
	root := t.TempDir()
	relFile := filepath.Join("bootstrap", "repositories.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import (
	orgrepo "zord/internal/repositories/organization"
	"zord/pkg/registry"
)

func registerRepositories(reg *registry.Registry) {
	reg.Provide(orgrepo.RegistryKey, orgrepo.NewOrganizationRepository(db))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("UnregisterRepository: %v", err)
	}
	got := readFile(t, absFile)
	mustNotContain(t, got,
		`orgrepo "zord/internal/repositories/organization"`,
		"orgrepo.RegistryKey",
		"orgrepo.NewOrganizationRepository",
	)
	mustParse(t, got)
}

// TestUnregisterRepository_WorksWithoutPackageOnDisk garante que o unregister
// não exige `internal/repositories/<snake>/` existir — o dev pode ter apagado
// o pacote antes do unregister (sequência válida).
func TestUnregisterRepository_WorksWithoutPackageOnDisk(t *testing.T) {
	root := t.TempDir()
	registerForUnregisterRepository(t, root, "Organization")
	// Remove o pacote do repositório do disco.
	if err := os.RemoveAll(filepath.Join(root, "internal", "repositories", "organization")); err != nil {
		t.Fatalf("rm repository pkg: %v", err)
	}

	if _, err := UnregisterRepository(UnregisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("UnregisterRepository: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))
	mustNotContain(t, got,
		`organizationrepo "zord/internal/repositories/organization"`,
		"organizationrepo.RegistryKey",
	)
	mustParse(t, got)
}
