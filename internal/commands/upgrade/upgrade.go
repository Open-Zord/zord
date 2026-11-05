// Package upgrade expõe o comando `zord upgrade`: migra o runtime do projeto da
// versão com que foi gerado até a versão do CLI, aplicando codemods versionados.
package upgrade

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Open-Zord/zord/internal/tools/upgrader"

	"github.com/spf13/cobra"
)

// Commands devolve o(s) comando(s) deste grupo para o root do CLI.
func Commands() []*cobra.Command {
	var repo string
	var dryRun, force bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Migra o runtime do projeto para a versão do CLI (codemods)",
		Long: "Aplica as migrations (codemods) pendentes entre a versão com que o projeto " +
			"foi gerado (.zord/version) e a versão deste CLI, transformando o runtime no " +
			"lugar e preservando suas edições. Muta código: commite antes para revisar o " +
			"diff (ou use --force). Use --dry-run para só listar o que seria aplicado.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			from, err := upgrader.ReadVersion(repo)
			if err != nil {
				return err
			}
			target := cmd.Root().Version

			pend := upgrader.Pending(from, target, upgrader.Registered())
			if len(pend) == 0 {
				fmt.Printf("Projeto já está atualizado (%s).\n", display(from))
				return nil
			}

			if dryRun {
				fmt.Printf("Migrations pendentes (%s → %s):\n", display(from), target)
				for _, m := range pend {
					fmt.Printf("  %s  %s\n", m.Version, m.Description)
				}
				return nil
			}

			if !force {
				if dirty, _ := gitDirty(repo); dirty {
					return fmt.Errorf("working tree sujo em %s — commite as mudanças antes (ou use --force)", repo)
				}
			}

			module, _ := moduleOf(repo)
			applied, applyErr := upgrader.ApplyPending(repo, module, from, target, upgrader.Registered())
			for _, m := range applied {
				fmt.Printf("  aplicada %s  %s\n", m.Version, m.Description)
			}
			if applyErr != nil {
				return applyErr
			}
			if err := upgrader.WriteVersion(repo, target); err != nil {
				return err
			}
			fmt.Printf("Projeto atualizado para %s.\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", ".", "raiz do projeto alvo")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "lista as migrations pendentes sem aplicar")
	cmd.Flags().BoolVar(&force, "force", false, "aplica mesmo com o working tree sujo")
	return []*cobra.Command{cmd}
}

// display formata a versão de origem para o usuário (vazia = projeto sem carimbo).
func display(v string) string {
	if v == "" {
		return "sem carimbo"
	}
	return v
}

// gitDirty reporta se o working tree em repo tem mudanças não commitadas. Se repo
// não é um repositório git (ou git não está disponível), devolve (false, err) e o
// chamador ignora a guarda.
func gitDirty(repo string) (bool, error) {
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// moduleOf lê o module path do go.mod na raiz do projeto.
func moduleOf(repo string) (string, error) {
	f, err := os.Open(filepath.Join(repo, "go.mod")) //nolint:gosec // path derives from the repo root
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("diretiva module não encontrada em go.mod")
}
