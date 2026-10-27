// Package pkgmanager expõe o comando `zord package`: add/list/remove de pacotes
// (baterias) do catálogo Zord num projeto.
package pkgmanager

import (
	"fmt"
	"os"
	"text/tabwriter"

	pm "github.com/Open-Zord/zord/internal/tools/pkgmanager"

	"github.com/spf13/cobra"
)

// Commands devolve o(s) comando(s) deste grupo para o root do CLI.
func Commands() []*cobra.Command {
	var repo string

	root := &cobra.Command{
		Use:   "package",
		Short: "Gerencia pacotes (baterias) do catálogo Zord no projeto",
		Long: "Instala, lista e remove pacotes do catálogo Zord. Um pacote é copiado " +
			"para pkg/<nome>/ (vira código seu) e conectado ao registry; o estado fica " +
			"em .zord/packages.lock.",
	}
	root.PersistentFlags().StringVar(&repo, "repo", ".", "raiz do projeto alvo")

	add := &cobra.Command{
		Use:   "add <nome>",
		Short: "Instala um pacote do catálogo no projeto",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			res, err := pm.Add(repo, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Pacote %q instalado em pkg/%s:\n", args[0], res.Package)
			for _, f := range res.Files {
				fmt.Printf("  + %s\n", f)
			}
			fmt.Printf("  ~ %s (registro no DI)\n", res.Bootstrap)
			fmt.Println("\nRode `zord arch-analyse` para validar a arquitetura.")
			return nil
		},
	}

	remove := &cobra.Command{
		Use:     "remove <nome>",
		Aliases: []string{"rm"},
		Short:   "Remove um pacote instalado",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := pm.Remove(repo, args[0]); err != nil {
				return err
			}
			fmt.Printf("Pacote %q removido (pkg/ e registro no DI).\n", args[0])
			return nil
		},
	}

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lista os pacotes do catálogo e seu status",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			entries, err := pm.List(repo)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			for _, e := range entries {
				status := "disponível"
				if e.Installed {
					status = "instalado"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Name, status, e.Description)
			}
			return tw.Flush()
		},
	}

	root.AddCommand(add, remove, list)
	return []*cobra.Command{root}
}
