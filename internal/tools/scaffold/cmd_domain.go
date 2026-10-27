package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Operações sobre artefatos de domínio",
	}
	cmd.AddCommand(newDomainCreateCmd())
	cmd.AddCommand(newDomainDeleteCmd())
	return cmd
}

func newDomainCreateCmd() *cobra.Command {
	var root string
	c := &cobra.Command{
		Use:   "create <Name>",
		Short: "Cria um novo arquivo de domínio com struct vazia",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := DomainCreate(args[0], DomainCreateOptions{Root: root})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "criado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newDomainDeleteCmd() *cobra.Command {
	var opts DomainDeleteOptions
	c := &cobra.Command{
		Use:   "delete <Domain>",
		Short: "Apaga internal/application/domain/<snake>/ se nenhuma camada downstream referencia",
		Long: `Remove a pasta do domínio em internal/application/domain/<snake>/, mas só
quando nenhuma camada downstream ainda referencia o domínio. Inverso simétrico
de ` + "`scaffold domain create`" + ` (NAVE-56): falha se a pasta já não existir.

Detecta dependências residuais e acumula TODAS num único erro (sem mutação
parcial) — apontando o comando que limpa cada camada:
  - internal/application/services/<snake>/...   → scaffold service delete
  - internal/repositories/<snake>/...           → scaffold repository delete
  - cmd/http/handlers/<snake>/...               → scaffold handler delete
  - cmd/http/routes/<snake>.go                  → scaffold route delete
  - bloco entre sentinelas pra table <snake>s   → scaffold derive schema --remove

Nunca faz cascade — apaga só a pasta do domínio, e só quando o resto já saiu.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := DomainDelete(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removido:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&opts.Table, "table", "", "nome da tabela usado pra checar a sentinela no HCL (default: snake_case(Domain)+\"s\")")
	c.Flags().StringVar(&opts.SchemaPath, "schema-path", "", "caminho do arquivo HCL (default: schemas/schema.my.hcl)")
	return c
}
