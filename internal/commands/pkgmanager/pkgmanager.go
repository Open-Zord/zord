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

	update := &cobra.Command{
		Use:   "update [<nome>]",
		Short: "Re-sincroniza pacote(s) instalado(s) com o catálogo (respeita edições locais)",
		Long: "Atualiza um pacote instalado (ou todos, sem argumento) para a versão do " +
			"catálogo. Arquivos não editados são sobrescritos; arquivos que você editou " +
			"ficam intactos e a versão nova vai para .zord/updates/<path>.new.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var names []string
			if len(args) == 1 {
				names = []string{args[0]}
			} else {
				ns, err := pm.InstalledNames(repo)
				if err != nil {
					return err
				}
				names = ns
			}
			if len(names) == 0 {
				fmt.Println("Nenhum pacote instalado.")
				return nil
			}
			for _, n := range names {
				res, err := pm.Update(repo, n)
				if err != nil {
					return err
				}
				printUpdate(res)
			}
			return nil
		},
	}

	root.AddCommand(add, remove, list, update)
	return []*cobra.Command{root}
}

// printUpdate resume o resultado de um update para o usuário.
func printUpdate(res pm.UpdateResult) {
	if !res.Changed() {
		fmt.Printf("%s: já está atualizado.\n", res.Name)
		return
	}
	fmt.Printf("%s:\n", res.Name)
	for _, f := range res.Overwritten {
		fmt.Printf("  ~ %s (atualizado)\n", f)
	}
	for _, f := range res.Added {
		fmt.Printf("  + %s (adicionado)\n", f)
	}
	for _, f := range res.Removed {
		fmt.Printf("  - %s (não existe mais no catálogo; mantido)\n", f)
	}
	for _, f := range res.Conflicts {
		fmt.Printf("  ! %s (editado localmente — veja .zord/updates/%s.new)\n", f, f)
	}
	if len(res.Conflicts) > 0 {
		fmt.Println("\nResolva os conflitos (.new) manualmente e rode `zord package update` de novo.")
	}
}
