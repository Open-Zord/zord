package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// SchemaUndriveOptions parametriza SchemaUnderive.
type SchemaUndriveOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "OrgMembership").
	Domain string
	// Table sobrescreve o nome da tabela removida. Vazio = snake_case(Domain) + "s".
	Table string
	// SchemaPath sobrescreve o caminho do arquivo HCL. Vazio = DefaultSchemaPath.
	SchemaPath string
}

// SchemaUnderive remove o bloco `table` do domínio do arquivo de schema Atlas,
// envolvido pelas sentinelas `# scaffold:generated <table>` / `# scaffold:end <table>`.
// Não lê o pacote Go do domínio — opera apenas sobre o nome da tabela (derivado
// de Domain ou override via Table). Retorna o caminho relativo do arquivo afetado.
// Falha se a sentinela estiver ausente ou parcial — re-executar após sucesso
// sempre falha (idempotência inversa).
func SchemaUnderive(opts SchemaUndriveOptions) (string, error) {
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
	relSchema := opts.SchemaPath
	if relSchema == "" {
		relSchema = DefaultSchemaPath
	}

	absSchema := filepath.Join(root, relSchema)
	//nolint:gosec // G304: path derives from SchemaUndriveOptions (validated) + repo constants
	raw, err := os.ReadFile(absSchema)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", relSchema, err)
	}

	patched, err := unpatch(raw, table)
	if err != nil {
		return "", fmt.Errorf("unpatch %s: %w", relSchema, err)
	}

	if err := os.WriteFile(absSchema, patched, 0o600); err != nil { //nolint:gosec // G703: path resolvido a partir do repo do scaffold local
		return "", fmt.Errorf("escrever %s: %w", relSchema, err)
	}
	return relSchema, nil
}
