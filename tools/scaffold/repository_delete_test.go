package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryDelete_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	// bootstrap sem wire-up — dev rodou unregister antes (ou nunca registrou).
	seedBootstrapRepositories(t, root)
	absDir := filepath.Join(root, "internal", "repositories", "organization")
	if _, err := os.Stat(absDir); err != nil {
		t.Fatalf("pré-condição falhou: %v", err)
	}

	rel, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"})
	if err != nil {
		t.Fatalf("RepositoryDelete: %v", err)
	}
	if want := filepath.Join("internal", "repositories", "organization"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	if _, err := os.Stat(absDir); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete: %v", err)
	}
}

func TestRepositoryDelete_HappyPath_CompoundDomain(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "OrgMembership")
	seedBootstrapRepositories(t, root)
	rel, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "OrgMembership"})
	if err != nil {
		t.Fatalf("RepositoryDelete: %v", err)
	}
	if want := filepath.Join("internal", "repositories", "org_membership"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete: %v", err)
	}
}

func TestRepositoryDelete_HappyPath_NoBootstrap(t *testing.T) {
	// bootstrap/repositories.go ausente = sem wire-up residual. Delete prossegue.
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")

	if _, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("RepositoryDelete: %v", err)
	}
}

func TestRepositoryDelete_HappyPath_BootstrapWithoutRegisterFunc(t *testing.T) {
	// bootstrap presente mas sem registerRepositories — estado consistente.
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	absFile := filepath.Join(root, "bootstrap", "repositories.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

var _ = registry.Registry{}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("RepositoryDelete: %v", err)
	}
}

func TestRepositoryDelete_FailsIfWireUpResidual(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	seedBootstrapRepositories(t, root)
	if _, err := RegisterRepository(RegisterRepositoryOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("RegisterRepository: %v", err)
	}

	_, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("RepositoryDelete: esperado erro pra wire-up residual, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"wire-up residual", "scaffold repository unregister Organization"} {
		if !strings.Contains(msg, want) {
			t.Errorf("erro %q não contém %q", msg, want)
		}
	}
	// Pasta intacta.
	absDir := filepath.Join(root, "internal", "repositories", "organization")
	if _, err := os.Stat(absDir); err != nil {
		t.Errorf("pasta removida em falha: %v", err)
	}
}

func TestRepositoryDelete_FailsIfWireUpResidual_ShortenedAlias(t *testing.T) {
	// Caso real: bootstrap/repositories.go com alias encurtado à mão.
	// findImportSpec pelo importPath ainda casa, e importIdent retorna o
	// alias real — a mensagem aponta o RegistryKey com esse alias.
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	absFile := filepath.Join(root, "bootstrap", "repositories.go")
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

	_, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("RepositoryDelete: esperado erro pra wire-up residual com alias curto, got nil")
	}
	if !strings.Contains(err.Error(), "orgrepo.RegistryKey") {
		t.Errorf("erro %q não referencia o alias real %q", err.Error(), "orgrepo.RegistryKey")
	}
}

func TestRepositoryDelete_FailsIfFolderMissing(t *testing.T) {
	root := t.TempDir()
	seedBootstrapRepositories(t, root)

	_, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatalf("RepositoryDelete: esperado erro pra pasta ausente, got nil")
	}
	if !strings.Contains(err.Error(), "não encontrado") {
		t.Errorf("erro %q não menciona 'não encontrado'", err.Error())
	}
}

func TestRepositoryDelete_AllowsResidualImportWithoutProvide(t *testing.T) {
	// Estado patológico: import sobrou no bootstrap mas a linha Provide não.
	// Como não há nada pra `repository unregister` desfazer (ele falharia em
	// "Provide ausente"), o delete prossegue. Dev limpa o import órfão depois,
	// ou o goimports faz isso na próxima edição.
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	absFile := filepath.Join(root, "bootstrap", "repositories.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import (
	organizationrepo "zord/internal/repositories/organization"
	"zord/pkg/registry"
)

var _ = organizationrepo.RegistryKey

func registerRepositories(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("RepositoryDelete: %v", err)
	}
}

func TestRepositoryDelete_FailsOnInvalidIdent(t *testing.T) {
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
			_, err := RepositoryDelete(RepositoryDeleteOptions{Domain: tc.domain})
			if err == nil {
				t.Fatalf("RepositoryDelete(%q): esperado erro, got nil", tc.domain)
			}
		})
	}
}

func TestRepositoryDelete_IsNotIdempotent(t *testing.T) {
	// Segundo delete falha porque a pasta sumiu — mesma convenção do resto
	// do scaffold (idempotência por falha).
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	seedBootstrapRepositories(t, root)
	if _, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("primeiro RepositoryDelete: %v", err)
	}
	if _, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"}); err == nil {
		t.Fatalf("segundo RepositoryDelete: esperado erro, got nil")
	}
}

func TestRepositoryDelete_PreservesSiblingRepositories(t *testing.T) {
	root := t.TempDir()
	seedRepositoryFile(t, root, "Organization")
	seedRepositoryFile(t, root, "PlatformUser")
	seedBootstrapRepositories(t, root)

	if _, err := RepositoryDelete(RepositoryDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("RepositoryDelete: %v", err)
	}
	// Organization removido.
	if _, err := os.Stat(filepath.Join(root, "internal", "repositories", "organization")); !os.IsNotExist(err) {
		t.Errorf("organization ainda existe: %v", err)
	}
	// PlatformUser intacto.
	if _, err := os.Stat(filepath.Join(root, "internal", "repositories", "platform_user")); err != nil {
		t.Errorf("platform_user foi removido: %v", err)
	}
}
