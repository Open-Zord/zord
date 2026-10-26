package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newProjectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projection",
		Short: "Operações sobre projection structs (tipos de retorno de queries agregadas)",
	}
	cmd.AddCommand(newProjectionCreateCmd())
	cmd.AddCommand(newProjectionFieldCmd())
	return cmd
}

func newProjectionCreateCmd() *cobra.Command {
	var root string
	c := &cobra.Command{
		Use:   "create <Domain> <ProjectionName>",
		Short: "Anexa uma struct projection vazia ao arquivo do domínio",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := ProjectionCreate(ProjectionCreateOptions{
				Root:           root,
				Domain:         args[0],
				ProjectionName: args[1],
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newProjectionFieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field",
		Short: "Operações granulares sobre campos de projection structs",
	}
	cmd.AddCommand(newProjectionFieldAddCmd())
	cmd.AddCommand(newProjectionFieldRemoveCmd())
	return cmd
}

type projectionFieldAddFlags struct {
	root    string
	tagDB   string
	noDBTag bool
}

func newProjectionFieldAddCmd() *cobra.Command {
	var fl projectionFieldAddFlags
	c := &cobra.Command{
		Use:   "add <Domain> <ProjectionName> <Field> <Type>",
		Short: "Adiciona um campo a uma projection struct",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("tag-db") && fl.noDBTag {
				return fmt.Errorf("--tag-db e --no-db-tag são incompatíveis")
			}
			path, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
				Root:           fl.root,
				Domain:         args[0],
				ProjectionName: args[1],
				FieldName:      args[2],
				TypeStr:        args[3],
				DBTagSet:       cmd.Flags().Changed("tag-db"),
				DBTagValue:     fl.tagDB,
				NoDBTag:        fl.noDBTag,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&fl.root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&fl.tagDB, "tag-db", "", "override do nome da coluna db (default: snake do campo)")
	c.Flags().BoolVar(&fl.noDBTag, "no-db-tag", false, "suprime a tag db (pra campos de struct composta que não vêm de StructScan)")
	return c
}

func newProjectionFieldRemoveCmd() *cobra.Command {
	var root string
	c := &cobra.Command{
		Use:   "remove <Domain> <ProjectionName> <Field>",
		Short: "Remove um campo de uma projection struct",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := ProjectionFieldRemove(ProjectionFieldRemoveOptions{
				Root:           root,
				Domain:         args[0],
				ProjectionName: args[1],
				FieldName:      args[2],
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}
