// Package newproject expõe o comando `zord new`, que gera um projeto a partir
// do runtime boilerplate embarcado.
package newproject

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Open-Zord/zord/internal/projectgen"
	"github.com/Open-Zord/zord/internal/skeleton"

	"github.com/spf13/cobra"
)

// Commands devolve o(s) comando(s) deste grupo para o root do CLI.
func Commands() []*cobra.Command {
	var module string
	var noTidy bool

	cmd := &cobra.Command{
		Use:   "new <caminho>",
		Short: "Cria um projeto novo a partir do runtime boilerplate do Zord",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dest := args[0]
			if module == "" {
				module = filepath.Base(filepath.Clean(dest))
			}
			if err := ensureEmpty(dest); err != nil {
				return err
			}
			fsys, err := skeleton.Runtime()
			if err != nil {
				return fmt.Errorf("carregando runtime embarcado: %w", err)
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			if err := projectgen.Generate(fsys, dest, module); err != nil {
				return fmt.Errorf("gerando projeto: %w", err)
			}
			if !noTidy {
				if err := goModTidy(dest); err != nil {
					fmt.Fprintf(os.Stderr, "aviso: go mod tidy falhou (%v); rode manualmente em %s\n", err, dest)
				}
			}
			printNextSteps(dest, module)
			return nil
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "module path do go.mod (default: nome do diretório)")
	cmd.Flags().BoolVar(&noTidy, "no-tidy", false, "pula o go mod tidy no projeto gerado")
	return []*cobra.Command{cmd}
}

// ensureEmpty falha se dest existe e não está vazio.
func ensureEmpty(dest string) error {
	entries, err := os.ReadDir(dest)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("destino %q já existe e não está vazio", dest)
	}
	return nil
}

// goModTidy roda `go mod tidy` no projeto gerado. GOWORK=off garante resolução
// standalone, mesmo se dest estiver dentro de um workspace.
func goModTidy(dest string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dest
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printNextSteps(dest, module string) {
	fmt.Printf("Projeto %q criado em %s\n\n", module, dest)
	fmt.Println("Próximos passos:")
	fmt.Printf("  cd %s\n", dest)
	fmt.Println("  docker compose up -d   # sobe MySQL + app")
	fmt.Println("  go run ./cmd/http")
}
