package scaffold

import (
	"strings"
	"testing"
)

func TestSchemaUnderive_RemovesBlockPreservingNeighbors(t *testing.T) {
	root := seedRoot(t)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive seed: %v", err)
	}
	// confirma que o bloco existe antes
	before := readSchema(t, root)
	if !strings.Contains(before, `table "widgets"`) {
		t.Fatalf("setup inválido: bloco widgets ausente antes do undrive")
	}

	if _, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaUnderive: %v", err)
	}
	got := readSchema(t, root)

	for _, banned := range []string{
		"# scaffold:generated widgets",
		"# scaffold:end widgets",
		`table "widgets"`,
	} {
		if strings.Contains(got, banned) {
			t.Errorf("conteúdo persistiu após undrive: %q\n%s", banned, got)
		}
	}
	// preserva o schema declaration original
	if !strings.HasPrefix(got, initialSchema) {
		t.Errorf("conteúdo fora da sentinela alterado; got:\n%s", got)
	}
}

func TestSchemaUnderive_CustomTableOverride(t *testing.T) {
	root := seedRoot(t)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget", Table: "custom_widgets"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	if _, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "Widget", Table: "custom_widgets"}); err != nil {
		t.Fatalf("SchemaUnderive: %v", err)
	}
	got := readSchema(t, root)
	if strings.Contains(got, "custom_widgets") {
		t.Errorf("tabela customizada persistiu; got:\n%s", got)
	}
}

func TestSchemaUnderive_FailsOnMissingSentinel(t *testing.T) {
	root := seedRoot(t)
	// sem rodar SchemaDerive antes — não há sentinela
	_, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "Widget"})
	if err == nil {
		t.Fatal("esperava erro de sentinela ausente; got nil")
	}
	if !strings.Contains(err.Error(), "sentinela ausente") {
		t.Errorf("erro inesperado: %v", err)
	}
}

func TestSchemaUnderive_FailsOnPartialSentinel(t *testing.T) {
	root := seedRoot(t)
	writeSchema(t, root, initialSchema+"\n# scaffold:generated widgets\ntable \"widgets\" { }\n")
	_, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "Widget"})
	if err == nil {
		t.Fatal("esperava erro de sentinela parcial; got nil")
	}
	if !strings.Contains(err.Error(), "sem :end") {
		t.Errorf("erro inesperado: %v", err)
	}
}

func TestSchemaUnderive_FailsOnInvalidDomainName(t *testing.T) {
	root := seedRoot(t)
	_, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "widget"})
	if err == nil {
		t.Fatal("esperava erro de nome inválido; got nil")
	}
}

func TestSchemaUnderive_IdempotentAfterRemoval(t *testing.T) {
	root := seedRoot(t)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	if _, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("primeiro SchemaUnderive: %v", err)
	}
	// re-rodar falha — sentinela já foi removida
	_, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "Widget"})
	if err == nil {
		t.Fatal("esperava erro no segundo undrive; got nil")
	}
	if !strings.Contains(err.Error(), "sentinela ausente") {
		t.Errorf("erro inesperado: %v", err)
	}
}

func TestSchemaUnderive_PreservesOutsideContent(t *testing.T) {
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
	writeSchema(t, root, initialSchema+extra)
	if _, err := SchemaDerive(SchemaDeriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaDerive: %v", err)
	}
	if _, err := SchemaUnderive(SchemaUndriveOptions{Root: root, Domain: "Widget"}); err != nil {
		t.Fatalf("SchemaUnderive: %v", err)
	}
	got := readSchema(t, root)
	if !strings.HasPrefix(got, initialSchema+extra) {
		t.Errorf("conteúdo fora da sentinela alterado; got:\n%s", got)
	}
	if strings.Contains(got, `table "widgets"`) {
		t.Errorf("bloco widgets persistiu; got:\n%s", got)
	}
}
