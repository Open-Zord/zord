// Package scaffold (área cmd_response) liga os subcomandos cobra das
// operações sobre o `response.go` do use case: adicionar/remover campos da
// struct `Response`.
package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResponseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "response",
		Short: "Operações sobre response.go de um use case (struct Response)",
	}
	cmd.AddCommand(newResponseFieldCmd())
	return cmd
}

func newResponseFieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field",
		Short: "Operações sobre campos da struct Response do response",
	}
	cmd.AddCommand(newResponseFieldAddCmd())
	cmd.AddCommand(newResponseFieldRemoveCmd())
	return cmd
}

func newResponseFieldAddCmd() *cobra.Command {
	var opts ResponseFieldAddOptions
	c := &cobra.Command{
		Use:   "add <Domain> <Verb> <Field> <Type>",
		Short: "Adiciona um campo à struct Response do response",
		Long: `Adiciona o campo à struct Response em internal/application/services/<snake_domain>/<snake_verb>/response.go.
Sempre com tag json:"<snake_field>".

O scaffold service create precisa ter rodado antes.`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			opts.FieldName = args[2]
			opts.TypeStr = args[3]
			path, err := ResponseFieldAdd(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newResponseFieldRemoveCmd() *cobra.Command {
	var opts ResponseFieldRemoveOptions
	c := &cobra.Command{
		Use:   "remove <Domain> <Verb> <Field>",
		Short: "Remove um campo da struct Response do response",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			opts.FieldName = args[2]
			path, err := ResponseFieldRemove(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}
