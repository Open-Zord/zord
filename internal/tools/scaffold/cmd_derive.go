package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDeriveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "derive",
		Short: "Deriva artefatos a partir do domínio (HCL, …)",
	}
	cmd.AddCommand(newDeriveSchemaCmd())
	return cmd
}

func newDeriveSchemaCmd() *cobra.Command {
	var opts SchemaDeriveOptions
	var remove bool
	c := &cobra.Command{
		Use:   "schema <Domain>",
		Short: "Regenera (ou remove) o bloco HCL Atlas do domínio",
		Long: `Regenera o bloco ` + "`table` " + `do domínio no arquivo de schema Atlas.
O bloco fica envolvido por sentinelas:

  # scaffold:generated <table>
  ...
  # scaffold:end <table>

Reexecutar substitui o bloco entre sentinelas; conteúdo fora é preservado.
Falha se já existir um bloco ` + "`table`" + ` com o mesmo nome fora de sentinela
(criado à mão).

Com --remove, apaga o bloco delimitado pelas sentinelas em vez de gerá-lo.
Falha se a sentinela estiver ausente ou parcial.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if remove {
				path, err := SchemaUnderive(SchemaUndriveOptions{
					Root:       opts.Root,
					Domain:     args[0],
					Table:      opts.Table,
					SchemaPath: opts.SchemaPath,
				})
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removido de:", path)
				return nil
			}
			opts.Domain = args[0]
			path, err := SchemaDerive(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&opts.Table, "table", "", "nome da tabela gerada (default: snake_case(Domain)+\"s\")")
	c.Flags().StringVar(&opts.SchemaPath, "schema-path", "", "caminho do arquivo HCL (default: "+DefaultSchemaPath+")")
	c.Flags().StringVar(&opts.SchemaName, "schema-name", "", "schema referenciado (default: "+DefaultSchemaName+") — ignorado com --remove")
	c.Flags().BoolVar(&remove, "remove", false, "remove o bloco delimitado pelas sentinelas em vez de gerar")
	return c
}
