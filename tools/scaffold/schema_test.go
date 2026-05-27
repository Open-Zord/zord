package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const initialSchema = `schema "zord" {
  charset = "utf8mb4"
  collate = "utf8mb4_0900_ai_ci"
}
`

// seedRoot prepara uma raiz com schema HCL inicial mais um domínio Widget
// com um conjunto base de campos.
func seedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeSchema(t, root, initialSchema)
	mustDomain(t, root, "Widget")
	mustField(t, root, "Widget", "ID", "string", []FieldTag{
		{Key: "db", Value: "id"}, {Key: "db_pk", Value: ""},
	})
	return root
}

func writeSchema(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, "schemas")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "schema.my.hcl"), []byte(content), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
}

func readSchema(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, DefaultSchemaPath))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(b)
}

func mustDomain(t *testing.T, root, name string) {
	t.Helper()
	if _, err := DomainCreate(name, DomainCreateOptions{Root: root}); err != nil {
		t.Fatalf("create domain %s: %v", name, err)
	}
}

func mustField(t *testing.T, root, dom, fname, ftype string, tags []FieldTag) {
	t.Helper()
	if _, err := FieldAdd(FieldAddOptions{
		Root: root, Domain: dom, FieldName: fname, TypeStr: ftype, Tags: tags,
	}); err != nil {
		t.Fatalf("add field %s.%s: %v", dom, fname, err)
	}
}

func mustRemoveField(t *testing.T, root, dom, fname string) {
	t.Helper()
	if _, err := FieldRemove(FieldRemoveOptions{Root: root, Domain: dom, FieldName: fname}); err != nil {
		t.Fatalf("remove field %s.%s: %v", dom, fname, err)
	}
}

// --- Cenários principais ---

func TestSchemaDerive_InitialGeneration(t *testing.T) {
	root := seedRoot(t)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	got := readSchema(t, root)

	for _, want := range []string{
		"# scaffold:generated widgets",
		"# scaffold:end widgets",
		`table "widgets"`,
		"schema = schema.zord",
		`column "id"`,
		"primary_key {",
		"[column.id]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q; got:\n%s", want, got)
		}
	}
	// preserva conteúdo original
	if !strings.HasPrefix(got, initialSchema) {
		t.Errorf("conteúdo inicial perdido; got prefix:\n%q", got[:min(len(got), 80)])
	}
}

func TestSchemaDerive_Idempotent(t *testing.T) {
	root := seedRoot(t)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("first SchemaDerive: %v", err)
	}
	first := readSchema(t, root)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("second SchemaDerive: %v", err)
	}
	second := readSchema(t, root)
	if first != second {
		t.Errorf("não idempotente:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestSchemaDerive_RegeneratesAfterFieldAdd(t *testing.T) {
	root := seedRoot(t)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("first SchemaDerive: %v", err)
	}
	mustField(t, root, "Widget", "Name", "string", []FieldTag{
		{Key: "db", Value: "name"},
		{Key: "db_size", Value: "100"},
	})
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("regen SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	if !strings.Contains(got, `column "name"`) {
		t.Errorf("coluna name ausente após regen; got:\n%s", got)
	}
	if !strings.Contains(got, "varchar(100)") {
		t.Errorf("db_size não aplicado; got:\n%s", got)
	}
	// só uma sentinela (não duplicou)
	if c := strings.Count(got, "# scaffold:generated widgets"); c != 1 {
		t.Errorf("sentinelas :generated = %d; esperado 1", c)
	}
	if c := strings.Count(got, "# scaffold:end widgets"); c != 1 {
		t.Errorf("sentinelas :end = %d; esperado 1", c)
	}
}

func TestSchemaDerive_RegeneratesAfterFieldRemove(t *testing.T) {
	root := seedRoot(t)
	mustField(t, root, "Widget", "Name", "string", []FieldTag{{Key: "db", Value: "name"}})
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("first SchemaDerive: %v", err)
	}
	mustRemoveField(t, root, "Widget", "Name")
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("regen SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	if strings.Contains(got, `column "name"`) {
		t.Errorf("coluna name persistiu após remove; got:\n%s", got)
	}
}

func TestSchemaDerive_NullabilityByPointer(t *testing.T) {
	root := seedRoot(t)
	mustField(t, root, "Widget", "DeletedAt", "*time.Time", []FieldTag{{Key: "db", Value: "deleted_at"}})
	mustField(t, root, "Widget", "CreatedAt", "time.Time", []FieldTag{{Key: "db", Value: "created_at"}})
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	// deleted_at -> null = true
	if !regionHasNull(got, "deleted_at", true) {
		t.Errorf("deleted_at deveria ser null = true; got:\n%s", got)
	}
	// created_at -> null = false
	if !regionHasNull(got, "created_at", false) {
		t.Errorf("created_at deveria ser null = false; got:\n%s", got)
	}
}

func TestSchemaDerive_ForeignKey(t *testing.T) {
	root := seedRoot(t)
	mustField(t, root, "Widget", "OrgID", "string", []FieldTag{
		{Key: "db", Value: "org_id"},
		{Key: "db_fk", Value: "organizations.id"},
	})
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	for _, want := range []string{
		`foreign_key "fk_widgets_org_id"`,
		"[column.org_id]",
		"[table.organizations.column.id]",
		"on_delete   = CASCADE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("FK ausente: %q; got:\n%s", want, got)
		}
	}
}

func TestSchemaDerive_IndexSimpleAndUnique(t *testing.T) {
	root := seedRoot(t)
	mustField(t, root, "Widget", "OrgID", "string", []FieldTag{
		{Key: "db", Value: "org_id"},
		{Key: "db_index", Value: "true"},
	})
	mustField(t, root, "Widget", "Slug", "string", []FieldTag{
		{Key: "db", Value: "slug"},
		{Key: "db_index", Value: "unique"},
	})
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	if !strings.Contains(got, `index "idx_widgets_org_id"`) {
		t.Errorf("index simples ausente; got:\n%s", got)
	}
	if !strings.Contains(got, `index "idx_widgets_slug_uq"`) {
		t.Errorf("index único ausente; got:\n%s", got)
	}
	if !strings.Contains(got, "unique  = true") {
		t.Errorf("atributo unique ausente; got:\n%s", got)
	}
}

func TestSchemaDerive_DBSizeArguments(t *testing.T) {
	root := seedRoot(t)
	mustField(t, root, "Widget", "Cost", "string", []FieldTag{
		{Key: "db", Value: "cost"},
		{Key: "db_type", Value: "decimal"},
		{Key: "db_size", Value: "16,6"},
	})
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	if !strings.Contains(got, "decimal(16, 6)") && !strings.Contains(got, "decimal(16,6)") {
		t.Errorf("decimal multi-arg ausente; got:\n%s", got)
	}
}

func TestSchemaDerive_PreservesOutsideSentinel(t *testing.T) {
	root := seedRoot(t)
	const extra = `

table "preexisting" {
  schema = schema.zord
  column "id" {
    type = varchar(26)
    null = false
  }
  primary_key {
    columns = [column.id]
  }
}
`
	// arquivo: schema inicial + tabela manual no fim
	writeSchema(t, root, initialSchema+extra)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	if !strings.HasPrefix(got, initialSchema+extra) {
		t.Errorf("conteúdo fora da sentinela alterado; got:\n%s", got)
	}
}

// --- Erros ---

func TestSchemaDerive_FailsOnPartialSentinel(t *testing.T) {
	root := seedRoot(t)
	writeSchema(t, root, initialSchema+"\n# scaffold:generated widgets\ntable \"widgets\" { }\n")
	_, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"})
	if err == nil {
		t.Fatal("esperava erro de sentinela parcial; got nil")
	}
	if !strings.Contains(err.Error(), "sem :end") {
		t.Errorf("erro inesperado: %v", err)
	}
}

func TestSchemaDerive_FailsOnUnwrappedTable(t *testing.T) {
	root := seedRoot(t)
	const handWritten = `
table "widgets" {
  schema = schema.zord
  column "id" {
    type = varchar(26)
    null = false
  }
  primary_key {
    columns = [column.id]
  }
}
`
	writeSchema(t, root, initialSchema+handWritten)
	_, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"})
	if err == nil {
		t.Fatal("esperava erro de table sem sentinela; got nil")
	}
	if !strings.Contains(err.Error(), "sem sentinela") {
		t.Errorf("erro inesperado: %v", err)
	}
}

func TestSchemaDerive_FailsOnInvalidDomainName(t *testing.T) {
	root := seedRoot(t)
	_, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "widget"})
	if err == nil {
		t.Fatal("esperava erro de nome inválido; got nil")
	}
}

func TestSchemaDerive_FailsOnMissingDomain(t *testing.T) {
	root := seedRoot(t)
	_, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Ghost"})
	if err == nil {
		t.Fatal("esperava erro de domínio inexistente; got nil")
	}
}

func TestSchemaDerive_FailsWhenStructHasNoColumns(t *testing.T) {
	root := seedRoot(t)
	mustDomain(t, root, "Empty")
	_, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Empty"})
	if err == nil {
		t.Fatal("esperava erro com struct sem campos db; got nil")
	}
}

func TestSchemaDerive_CustomTableName(t *testing.T) {
	root := seedRoot(t)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget", Table: "custom_widgets"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	got := readSchema(t, root)
	if !strings.Contains(got, `table "custom_widgets"`) {
		t.Errorf("tabela customizada ausente; got:\n%s", got)
	}
	if !strings.Contains(got, "# scaffold:generated custom_widgets") {
		t.Errorf("sentinela com nome customizado ausente; got:\n%s", got)
	}
}

// --- helpers ---

// regionHasNull verifica se o bloco `column "<colName>" { ... }` mais próximo
// contém `null = <expect>`. Tolerante a whitespace.
func regionHasNull(content, colName string, expect bool) bool {
	_, rest, ok := strings.Cut(content, `column "`+colName+`"`)
	if !ok {
		return false
	}
	block, _, ok := strings.Cut(rest, "}")
	if !ok {
		return false
	}
	want := "null = false"
	if expect {
		want = "null = true"
	}
	return strings.Contains(block, want)
}
