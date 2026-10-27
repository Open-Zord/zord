package migrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewMigrator(t *testing.T) {
	m := NewMigrator("u:p@tcp(db:3306)/app", "u:p@tcp(dev:3306)/dev", "app")
	if m.dsn != "u:p@tcp(db:3306)/app" || m.dsnTest != "u:p@tcp(dev:3306)/dev" || m.databaseName != "app" {
		t.Fatalf("campos não armazenados: %+v", m)
	}
}

// Sem tenant: Vars/Schema usam o databaseName; URLs derivam dos DSNs.
func TestApplyParams_NoTenant(t *testing.T) {
	m := NewMigrator("prod-dsn", "dev-dsn", "app")
	p := m.applyParams("", "/tmp/wd")

	if p.DevURL != "mysql://dev-dsn" {
		t.Errorf("DevURL = %q", p.DevURL)
	}
	if p.URL != "mysql://prod-dsn" {
		t.Errorf("URL = %q", p.URL)
	}
	if p.To != "file:///tmp/wd" {
		t.Errorf("To = %q", p.To)
	}
	if got := p.Vars["database"]; got != "app" {
		t.Errorf("Vars[database] = %v; want app", got)
	}
	if len(p.Schema) != 1 || p.Schema[0] != "app" {
		t.Errorf("Schema = %v; want [app]", p.Schema)
	}
}

// Com tenant: tanto Vars quanto Schema passam a mirar o tenant, não o default.
func TestApplyParams_Tenant(t *testing.T) {
	m := NewMigrator("prod-dsn", "dev-dsn", "app")
	p := m.applyParams("tenant_x", "/tmp/wd")

	if got := p.Vars["database"]; got != "tenant_x" {
		t.Errorf("Vars[database] = %v; want tenant_x", got)
	}
	if len(p.Schema) != 1 || p.Schema[0] != "tenant_x" {
		t.Errorf("Schema = %v; want [tenant_x]", p.Schema)
	}
	// DSNs não mudam com tenant.
	if p.URL != "mysql://prod-dsn" || p.DevURL != "mysql://dev-dsn" {
		t.Errorf("URLs não deveriam mudar com tenant: URL=%q DevURL=%q", p.URL, p.DevURL)
	}
}

func TestInspectURL(t *testing.T) {
	if got := inspectURL("dsn", ""); got != "mysql://dsn" {
		t.Errorf("inspectURL sem database = %q; want mysql://dsn", got)
	}
	if got := inspectURL("dsn", "app"); got != "mysql://dsn/app" {
		t.Errorf("inspectURL com database = %q; want mysql://dsn/app", got)
	}
}

// Grava no caminho esperado, com o conteúdo e permissão 0o600.
func TestWriteSchema(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("schemas", 0o750); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}

	const hcl = "schema \"app\" {}\n"
	out, err := writeSchema("app", hcl)
	if err != nil {
		t.Fatalf("writeSchema: %v", err)
	}
	if out != "schemas/app.my.hcl" {
		t.Errorf("out = %q; want schemas/app.my.hcl", out)
	}

	got, err := os.ReadFile(filepath.Join(dir, out))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != hcl {
		t.Errorf("conteúdo = %q; want %q", got, hcl)
	}

	info, err := os.Stat(filepath.Join(dir, out))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o; want 600", perm)
	}
}

// Sem o diretório schemas/, a escrita falha e o erro é embrulhado (não engolido).
func TestWriteSchema_ErrorWrapped(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // sem mkdir schemas/
	_, err := writeSchema("app", "x")
	if err == nil || !strings.Contains(err.Error(), "gravar schemas/app.my.hcl") {
		t.Fatalf("esperado erro embrulhado de gravação, got: %v", err)
	}
}

// client() falha de forma clara quando o HCL do schema não existe (working dir
// não carrega) — prova a propagação de erro em vez do antigo fmt.Printf+return.
func TestClient_MissingHCL(t *testing.T) {
	t.Chdir(t.TempDir()) // sem schemas/schema.my.hcl
	m := NewMigrator("dsn", "dev", "app")
	_, _, err := m.client()
	if err == nil {
		t.Fatal("esperado erro quando o HCL do schema não existe")
	}
	if !strings.Contains(err.Error(), "working dir") {
		t.Errorf("erro não menciona working dir: %v", err)
	}
}
