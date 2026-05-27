package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterRepository_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	seedBootstrapRepositories(t, root)

	rel, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err != nil {
		t.Fatalf("RegisterRepository: %v", err)
	}
	if want := filepath.Join("bootstrap", "repositories.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`organizationrepo "zord/internal/repositories/organization"`,
		"reg.Provide(organizationrepo.RegistryKey, organizationrepo.NewOrganizationRepository(db))",
	)
	mustParse(t, got)
}

func TestRegisterRepository_HappyPath_CompoundDomain(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "OrgMembership")
	seedBootstrapRepositories(t, root)

	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "OrgMembership"}); err != nil {
		t.Fatalf("RegisterRepository: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))
	mustContain(t, got,
		`orgmembershiprepo "zord/internal/repositories/org_membership"`,
		"reg.Provide(orgmembershiprepo.RegistryKey, orgmembershiprepo.NewOrgMembershipRepository(db))",
	)
	mustParse(t, got)
}

func TestRegisterRepository_FailsIfRepositoryFileMissing(t *testing.T) {
	root := t.TempDir()
	seedBootstrapRepositories(t, root)
	// arquivo do repository não criado

	_, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("RegisterRepository: esperado erro pra repository ausente")
	}
	if !strings.Contains(err.Error(), "repository") {
		t.Errorf("erro %q não menciona repository", err.Error())
	}
}

func TestRegisterRepository_FailsIfRegistryKeyMissing(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFileWithoutRegistryKey(t, root, "Organization")
	seedBootstrapRepositories(t, root)

	_, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil || !strings.Contains(err.Error(), "RegistryKey") {
		t.Fatalf("RegisterRepository: esperado erro mencionando RegistryKey, got: %v", err)
	}
}

func TestRegisterRepository_FailsIfConstructorMissing(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFileWithoutConstructor(t, root, "Organization")
	seedBootstrapRepositories(t, root)

	_, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil || !strings.Contains(err.Error(), "NewOrganizationRepository") {
		t.Fatalf("RegisterRepository: esperado erro mencionando NewOrganizationRepository, got: %v", err)
	}
}

func TestRegisterRepository_FailsIfImportAlreadyPresent(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	seedBootstrapRepositories(t, root)

	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("primeiro RegisterRepository: %v", err)
	}
	_, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("segundo RegisterRepository idêntico: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já presente") {
		t.Errorf("erro %q não menciona 'já presente'", err.Error())
	}
}

func TestRegisterRepository_FailsIfProvideCallAlreadyPresent(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	// bootstrap/repositories.go pré-populado com a linha de Provide mas SEM o
	// import — caso patológico pra garantir que a checagem de Provide funciona
	// independentemente do estado do bloco de imports.
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

	_, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("RegisterRepository: esperado erro pra Provide já presente, got nil")
	}
	if !strings.Contains(err.Error(), "já presente") {
		t.Errorf("erro %q não menciona 'já presente'", err.Error())
	}
}

func TestRegisterRepository_FailsOnInvalidIdent(t *testing.T) {
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
			_, err := RegisterRepository(RegisterRepositoryOptions{Domain: tc.domain})
			if err == nil {
				t.Fatalf("RegisterRepository(%q): esperado erro, got nil", tc.domain)
			}
		})
	}
}

func TestRegisterRepository_FailsIfBootstrapMissing(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	// bootstrap/repositories.go ausente

	_, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("RegisterRepository: esperado erro pra bootstrap ausente")
	}
}

func TestRegisterRepository_FailsIfRegisterFuncMissing(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
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

	_, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"})
	if err == nil || !strings.Contains(err.Error(), registerRepositoriesFunc) {
		t.Fatalf("RegisterRepository: esperado erro mencionando %s, got: %v", registerRepositoriesFunc, err)
	}
}

func TestRegisterRepository_DoesNotMutateOnFailure(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	seedBootstrapRepositories(t, root)

	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("primeiro RegisterRepository: %v", err)
	}
	before := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))

	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"}); err == nil {
		t.Fatalf("segundo RegisterRepository: esperado erro, got nil")
	}
	after := readFile(t, filepath.Join(root, "bootstrap", "repositories.go"))
	if before != after {
		t.Fatalf("arquivo mutado após falha:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

// --- seeding helpers ---

// seedRepositoryFile grava um arquivo de repositório mínimo em
// `internal/repositories/<snake>/<snake>.go` com pacote `<snake>_repository`,
// `const RegistryKey` e `New<Pascal>RegisterRepository(db *sqlx.DB)`. É o estado
// canônico (NAVE-58 + RegistryKey manual) que `repository register` espera.
func seedRepositoryFile(t *testing.T, root, pascal string) {
	t.Helper()
	snake := ToSnake(pascal)
	absDir := filepath.Join(root, "internal/repositories", snake)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fmt.Sprintf(`package %s_repository

import "github.com/jmoiron/sqlx"

const RegistryKey = %q

type %sRepository struct {
	db *sqlx.DB
}

func New%sRepository(db *sqlx.DB) *%sRepository {
	return &%sRepository{db: db}
}
`, snake, ToLowerCamel(pascal)+"RegisterRepository", pascal, pascal, pascal, pascal)
	absFile := filepath.Join(absDir, snake+".go")
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
}

func seedRepositoryFileWithoutRegistryKey(t *testing.T, root, pascal string) {
	t.Helper()
	snake := ToSnake(pascal)
	absDir := filepath.Join(root, "internal/repositories", snake)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fmt.Sprintf(`package %s_repository

import "github.com/jmoiron/sqlx"

type %sRepository struct {
	db *sqlx.DB
}

func New%sRepository(db *sqlx.DB) *%sRepository {
	return &%sRepository{db: db}
}
`, snake, pascal, pascal, pascal, pascal)
	absFile := filepath.Join(absDir, snake+".go")
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
}

func seedRepositoryFileWithoutConstructor(t *testing.T, root, pascal string) {
	t.Helper()
	snake := ToSnake(pascal)
	absDir := filepath.Join(root, "internal/repositories", snake)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fmt.Sprintf(`package %s_repository

import "github.com/jmoiron/sqlx"

const RegistryKey = %q

type %sRepository struct {
	db *sqlx.DB
}
`, snake, ToLowerCamel(pascal)+"RegisterRepository", pascal)
	absFile := filepath.Join(absDir, snake+".go")
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
}

// seedBootstrapRepositories grava um `bootstrap/repositories.go` mínimo: a
// função `registerRepositories(reg *registry.Registry)` com corpo vazio.
func seedBootstrapRepositories(t *testing.T, root string) {
	t.Helper()
	relFile := filepath.Join("bootstrap", "repositories.go")
	absFile := filepath.Join(root, relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerRepositories(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed bootstrap/repositories.go: %v", err)
	}
}
