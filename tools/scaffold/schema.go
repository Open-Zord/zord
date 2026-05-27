// Package scaffold (área schema) deriva o bloco HCL Atlas de uma tabela a partir de um
// domínio Go. O bloco gerado fica envolvido por sentinelas
// `# scaffold:generated <table>` / `# scaffold:end <table>`; reexecuções
// substituem o bloco entre sentinelas e preservam o restante do arquivo.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultSchemaPath é o caminho, relativo à raiz do repositório, do arquivo
// de schema Atlas. Pode ser sobrescrito via SchemaDeriveOptions.SchemaPath.
const DefaultSchemaPath = "schemas/schema.my.hcl"

// DefaultSchemaName é o nome do schema referenciado em `schema = schema.<nome>`.
const DefaultSchemaName = "zord"

// SchemaDeriveOptions parametriza SchemaDerive.
type SchemaDeriveOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "OrgMembership").
	Domain string
	// Table sobrescreve o nome da tabela gerada. Vazio = snake_case(Domain) + "s".
	Table string
	// SchemaPath sobrescreve o caminho do arquivo HCL. Vazio = DefaultSchemaPath.
	SchemaPath string
	// SchemaName sobrescreve o schema referenciado. Vazio = DefaultSchemaName.
	SchemaName string
}

// SchemaDerive lê o domínio em internal/application/domain/<snake>/<snake>.go e
// regenera o bloco `table` do domínio no arquivo de schema Atlas.
// Retorna o caminho relativo do arquivo de schema afetado.
func SchemaDerive(opts SchemaDeriveOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	table := opts.Table
	if table == "" {
		table = ToSnake(opts.Domain) + "s"
	}
	schemaName := opts.SchemaName
	if schemaName == "" {
		schemaName = DefaultSchemaName
	}
	relSchema := opts.SchemaPath
	if relSchema == "" {
		relSchema = DefaultSchemaPath
	}

	fields, err := readDomain(root, opts.Domain)
	if err != nil {
		return "", err
	}

	block, err := buildTable(table, schemaName, fields)
	if err != nil {
		return "", err
	}

	absSchema := filepath.Join(root, relSchema)
	//nolint:gosec // G304: path derives from SchemaDeriveOptions (validated) + repo constants
	raw, err := os.ReadFile(absSchema)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", relSchema, err)
	}

	patched, err := patch(raw, table, block)
	if err != nil {
		return "", fmt.Errorf("patch %s: %w", relSchema, err)
	}

	if err := os.WriteFile(absSchema, patched, 0o600); err != nil { //nolint:gosec // G703: path resolvido a partir do repo do scaffold local
		return "", fmt.Errorf("escrever %s: %w", relSchema, err)
	}
	return relSchema, nil
}
