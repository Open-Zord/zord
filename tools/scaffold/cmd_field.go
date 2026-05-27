package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newFieldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field",
		Short: "Operações granulares sobre campos de structs de domínio",
	}
	cmd.AddCommand(newFieldAddCmd())
	cmd.AddCommand(newFieldRemoveCmd())
	return cmd
}

type fieldAddFlags struct {
	root     string
	db       string
	json     string
	validate string
	dbType   string
	dbSize   string
	dbPK     bool
	dbFK     string
	dbIndex  string
}

func newFieldAddCmd() *cobra.Command {
	var fl fieldAddFlags
	c := &cobra.Command{
		Use:   "add <Domain> <Field> <Type>",
		Short: "Adiciona um campo à struct de um domínio",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := FieldAdd(FieldAddOptions{
				Root:      fl.root,
				Domain:    args[0],
				FieldName: args[1],
				TypeStr:   args[2],
				Tags:      tagsFromFlags(cmd, &fl),
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&fl.root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&fl.db, "tag-db", "", "valor da tag db")
	c.Flags().StringVar(&fl.json, "tag-json", "", "valor da tag json")
	c.Flags().StringVar(&fl.validate, "tag-validate", "", "valor da tag validate")
	c.Flags().StringVar(&fl.dbType, "tag-db-type", "", "valor da tag db_type")
	c.Flags().StringVar(&fl.dbSize, "tag-db-size", "", "valor da tag db_size")
	c.Flags().BoolVar(&fl.dbPK, "tag-db-pk", false, "marca campo como primary key (tag db_pk)")
	c.Flags().StringVar(&fl.dbFK, "tag-db-fk", "", "valor da tag db_fk (ex.: organizations.id)")
	c.Flags().StringVar(&fl.dbIndex, "tag-db-index", "", "valor da tag db_index (ex.: unique)")
	return c
}

func tagsFromFlags(cmd *cobra.Command, fl *fieldAddFlags) []FieldTag {
	type src struct {
		key, val string
		set      bool
	}
	srcs := []src{
		{"db", fl.db, cmd.Flags().Changed("tag-db")},
		{"json", fl.json, cmd.Flags().Changed("tag-json")},
		{"validate", fl.validate, cmd.Flags().Changed("tag-validate")},
		{"db_type", fl.dbType, cmd.Flags().Changed("tag-db-type")},
		{"db_size", fl.dbSize, cmd.Flags().Changed("tag-db-size")},
		{"db_pk", "", fl.dbPK},
		{"db_fk", fl.dbFK, cmd.Flags().Changed("tag-db-fk")},
		{"db_index", fl.dbIndex, cmd.Flags().Changed("tag-db-index")},
	}
	tags := make([]FieldTag, 0, len(srcs))
	for _, s := range srcs {
		if !s.set {
			continue
		}
		tags = append(tags, FieldTag{Key: s.key, Value: s.val})
	}
	return tags
}

func newFieldRemoveCmd() *cobra.Command {
	var root string
	c := &cobra.Command{
		Use:   "remove <Domain> <Field>",
		Short: "Remove um campo da struct de um domínio",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := FieldRemove(FieldRemoveOptions{
				Root:      root,
				Domain:    args[0],
				FieldName: args[1],
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
