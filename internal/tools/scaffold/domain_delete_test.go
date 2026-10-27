package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFileTree cria todos os arquivos (parents inclusos) no root indicado.
// Helper para preparar fixtures com pasta de domínio + deps opcionais.
func writeFileTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
	}
}

// seedDomainStub cria a pasta canônica do domínio com um arquivo source mínimo.
func seedDomainStub(t *testing.T, root, snake string) string {
	t.Helper()
	rel := filepath.Join(domainBasePath, snake, snake+".go")
	writeFileTree(t, root, map[string]string{
		rel: "package " + snake + "\n\ntype Stub struct{}\n",
	})
	return filepath.Join(root, domainBasePath, snake)
}

func TestDomainDelete_HappyPath(t *testing.T) {
	root := t.TempDir()
	absDir := seedDomainStub(t, root, "organization")

	rel, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err != nil {
		t.Fatalf("DomainDelete: %v", err)
	}
	want := filepath.Join(domainBasePath, "organization")
	if rel != want {
		t.Fatalf("rel = %q; want %q", rel, want)
	}
	if _, err := os.Stat(absDir); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete: %v", err)
	}
}

func TestDomainDelete_PreserveSiblings(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	// irmão que precisa sobreviver
	sibling := filepath.Join(root, domainBasePath, "user", "user.go")
	writeFileTree(t, root, map[string]string{
		filepath.Join(domainBasePath, "user", "user.go"): "package user\n",
	})

	if _, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("DomainDelete: %v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Errorf("irmão removido por engano: %v", err)
	}
}

func TestDomainDelete_InvalidName(t *testing.T) {
	root := t.TempDir()
	cases := []string{"", "foo", "1Foo", "Foo-Bar"}
	for _, n := range cases {
		if _, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: n}); err == nil {
			t.Errorf("DomainDelete(%q): want error, got nil", n)
		}
	}
}

func TestDomainDelete_NotExists(t *testing.T) {
	root := t.TempDir()
	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDomainDelete_NotADirectory(t *testing.T) {
	root := t.TempDir()
	// cria um arquivo no lugar onde deveria haver a pasta do domínio
	relFile := filepath.Join(domainBasePath, "organization")
	writeFileTree(t, root, map[string]string{relFile: "garbage"})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "não é diretório") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDomainDelete_ServiceResidual(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	writeFileTree(t, root, map[string]string{
		filepath.Join(servicesBasePath, "organization", "create", "service.go"): "package create\n",
	})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "services/organization") {
		t.Errorf("erro não menciona services: %v", err)
	}
	if !strings.Contains(msg, "service delete") {
		t.Errorf("erro não sugere service delete: %v", err)
	}
	// Garante que NÃO mutou disco — pasta do domínio ainda existe.
	if _, err := os.Stat(filepath.Join(root, domainBasePath, "organization")); err != nil {
		t.Errorf("pasta do domínio sumiu apesar do erro: %v", err)
	}
}

func TestDomainDelete_RepositoryResidual(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	writeFileTree(t, root, map[string]string{
		filepath.Join(repositoriesBasePath, "organization", "organization.go"): "package organization_repository\n",
	})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "repositories/organization") {
		t.Errorf("erro não menciona repositories: %v", err)
	}
	if !strings.Contains(err.Error(), "repository delete") {
		t.Errorf("erro não sugere repository delete: %v", err)
	}
}

func TestDomainDelete_HandlerResidual(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	writeFileTree(t, root, map[string]string{
		filepath.Join(handlersBasePath, "organization", "create", "handler.go"): "package create\n",
	})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "handlers/organization") {
		t.Errorf("erro não menciona handlers: %v", err)
	}
	if !strings.Contains(err.Error(), "handler delete") {
		t.Errorf("erro não sugere handler delete: %v", err)
	}
}

func TestDomainDelete_RouteResidual(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	writeFileTree(t, root, map[string]string{
		filepath.Join(routesBasePath, "organization.go"): "package routes\n",
	})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "routes/organization.go") {
		t.Errorf("erro não menciona routes file: %v", err)
	}
	if !strings.Contains(err.Error(), "route delete") {
		t.Errorf("erro não sugere route delete: %v", err)
	}
}

func TestDomainDelete_SchemaSentinelResidual(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	hcl := `schema "zord" { charset = "utf8mb4" }
# scaffold:generated organizations
table "organizations" {
  schema = schema.zord
}
# scaffold:end organizations
`
	writeFileTree(t, root, map[string]string{DefaultSchemaPath: hcl})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "schemas/schema.my.hcl") {
		t.Errorf("erro não menciona schema path: %v", err)
	}
	if !strings.Contains(err.Error(), "table organizations") {
		t.Errorf("erro não menciona table: %v", err)
	}
	if !strings.Contains(err.Error(), "derive schema") {
		t.Errorf("erro não sugere derive schema: %v", err)
	}
}

func TestDomainDelete_SchemaCustomTable(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	hcl := `# scaffold:generated tenants
table "tenants" {
  schema = schema.zord
}
# scaffold:end tenants
`
	writeFileTree(t, root, map[string]string{DefaultSchemaPath: hcl})

	// Sem --table o default seria "organizations"; nada deve ser detectado.
	if _, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("DomainDelete sem --table custom: %v", err)
	}

	// Re-seeda e tenta com --table custom — agora detecta.
	seedDomainStub(t, root, "organization")
	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization", Table: "tenants"})
	if err == nil {
		t.Fatal("want error com --table tenants, got nil")
	}
	if !strings.Contains(err.Error(), "table tenants") {
		t.Errorf("erro não menciona tabela custom: %v", err)
	}
}

func TestDomainDelete_SchemaPathOverride(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	customPath := "infra/atlas.hcl"
	hcl := `# scaffold:generated organizations
table "organizations" {}
# scaffold:end organizations
`
	writeFileTree(t, root, map[string]string{customPath: hcl})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization", SchemaPath: customPath})
	if err == nil {
		t.Fatal("want error com --schema-path custom, got nil")
	}
	if !strings.Contains(err.Error(), customPath) {
		t.Errorf("erro não menciona schema path custom: %v", err)
	}
}

func TestDomainDelete_SchemaAbsent_NoFalsePositive(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	// Sem schema file — não deve erro.
	if _, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"}); err != nil {
		t.Fatalf("DomainDelete: %v", err)
	}
}

func TestDomainDelete_AllResidualsAggregated(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	hcl := `# scaffold:generated organizations
table "organizations" {}
# scaffold:end organizations
`
	writeFileTree(t, root, map[string]string{
		filepath.Join(servicesBasePath, "organization", "create", "service.go"): "package create\n",
		filepath.Join(repositoriesBasePath, "organization", "organization.go"):  "package organization_repository\n",
		filepath.Join(handlersBasePath, "organization", "create", "handler.go"): "package create\n",
		filepath.Join(routesBasePath, "organization.go"):                        "package routes\n",
		DefaultSchemaPath: hcl,
	})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	msg := err.Error()
	expects := []string{
		"5 dependência",
		"services/organization",
		"repositories/organization",
		"handlers/organization",
		"routes/organization.go",
		"schemas/schema.my.hcl",
	}
	for _, want := range expects {
		if !strings.Contains(msg, want) {
			t.Errorf("erro não contém %q; got:\n%s", want, msg)
		}
	}
}

func TestDomainDelete_SchemaScanError(t *testing.T) {
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	// sentinela órfã quebra scanSentinels e deve propagar
	hcl := `# scaffold:generated organizations
table "organizations" {}
`
	writeFileTree(t, root, map[string]string{DefaultSchemaPath: hcl})

	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "sentinela") {
		t.Errorf("erro não menciona sentinela: %v", err)
	}
}

func TestDomainDelete_ServicePathTypeMismatch(t *testing.T) {
	// services/<snake> existe como ARQUIVO em vez de diretório — sinal de
	// estado corrompido. Deve propagar erro explícito (sem cair no path normal).
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	writeFileTree(t, root, map[string]string{
		filepath.Join(servicesBasePath, "organization"): "garbage",
	})
	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "não é diretório") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDomainDelete_RoutePathTypeMismatch(t *testing.T) {
	// routes/<snake>.go existe como DIRETÓRIO em vez de arquivo.
	root := t.TempDir()
	seedDomainStub(t, root, "organization")
	if err := os.MkdirAll(filepath.Join(root, routesBasePath, "organization.go"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := DomainDelete(DomainDeleteOptions{Root: root, Domain: "Organization"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "não é arquivo") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDomainDelete_DefaultRoot_CWD(t *testing.T) {
	// Cobre o ramo `if root == "" { root = "." }` rodando com cwd no tempdir.
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	seedDomainStub(t, ".", "organization")
	if _, err := DomainDelete(DomainDeleteOptions{Domain: "Organization"}); err != nil {
		t.Fatalf("DomainDelete: %v", err)
	}
}
