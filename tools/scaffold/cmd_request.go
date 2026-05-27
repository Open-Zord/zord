// Package scaffold (área cmd_request) liga os subcomandos cobra das
// operações sobre o `request.go` do use case: adicionar/remover campos do
// `Data` e ligar/desligar validação na struct `Request`.
package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "request",
		Short: "Operações sobre request.go de um use case (Data + validator)",
	}
	cmd.AddCommand(newRequestFieldCmd())
	cmd.AddCommand(newRequestValidatorCmd())
	return cmd
}

func newRequestFieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field",
		Short: "Operações sobre campos da struct Data do request",
	}
	cmd.AddCommand(newRequestFieldAddCmd())
	cmd.AddCommand(newRequestFieldRemoveCmd())
	return cmd
}

func newRequestFieldAddCmd() *cobra.Command {
	var opts RequestFieldAddOptions
	c := &cobra.Command{
		Use:   "add <Domain> <Verb> <Field> <Type>",
		Short: "Adiciona um campo à struct Data do request",
		Long: `Adiciona o campo à struct Data em internal/application/services/<snake_domain>/<snake_verb>/request.go.

Tags geradas:
  - sempre json:"<snake_field>"
  - quando --validate=... é passado: validate:"<value>"

O scaffold service create precisa ter rodado antes.`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			opts.FieldName = args[2]
			opts.TypeStr = args[3]
			path, err := RequestFieldAdd(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&opts.Validate, "validate", "", "valor da tag validate (ex.: required,email)")
	return c
}

func newRequestFieldRemoveCmd() *cobra.Command {
	var opts RequestFieldRemoveOptions
	c := &cobra.Command{
		Use:   "remove <Domain> <Verb> <Field>",
		Short: "Remove um campo da struct Data do request",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			opts.FieldName = args[2]
			path, err := RequestFieldRemove(opts)
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

func newRequestValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validator",
		Short: "Liga/desliga validação no request.go do use case",
	}
	cmd.AddCommand(newRequestValidatorSetCmd())
	cmd.AddCommand(newRequestValidatorUnsetCmd())
	return cmd
}

func newRequestValidatorSetCmd() *cobra.Command {
	var opts RequestValidatorOptions
	c := &cobra.Command{
		Use:   "set <Domain> <Verb>",
		Short: "Liga validator no request.go (regenera o arquivo na variante validada)",
		Long: `Regenera internal/application/services/<snake_domain>/<snake_verb>/request.go
preservando os campos de Data e adicionando:
  - campo validator services.Validator na struct Request
  - parâmetro validator em NewRequest e entrada no return literal
  - body de Validate que itera sobre r.validator.ValidateStruct(r.Data)
  - import de <module>/internal/application/services

Falha se o validator já estiver configurado.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			path, err := RequestValidatorSet(opts)
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

func newRequestValidatorUnsetCmd() *cobra.Command {
	var opts RequestValidatorOptions
	c := &cobra.Command{
		Use:   "unset <Domain> <Verb>",
		Short: "Desliga validator no request.go (regenera o arquivo sem validator)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			path, err := RequestValidatorUnset(opts)
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
