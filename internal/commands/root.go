// Package commands monta o CLI único `zord`. Cada subcomando vive em
// internal/commands/<nome> e expõe Commands() []*cobra.Command. Adicionar um
// comando = criar o pacote + acrescentar seu Commands na lista de NewRootCmd.
package commands

import (
	"github.com/Open-Zord/zord/internal/commands/archanalyse"
	"github.com/Open-Zord/zord/internal/commands/mcp"
	"github.com/Open-Zord/zord/internal/commands/migrate"
	"github.com/Open-Zord/zord/internal/commands/newproject"
	"github.com/Open-Zord/zord/internal/commands/pkgmanager"
	"github.com/Open-Zord/zord/internal/commands/scaffold"

	"github.com/spf13/cobra"
)

// providers é o ponto único de registro. A ordem define a ordem no --help.
var providers = []func() []*cobra.Command{
	newproject.Commands,
	scaffold.Commands,
	archanalyse.Commands,
	migrate.Commands,
	pkgmanager.Commands,
	mcp.Commands,
}

// NewRootCmd constrói o comando raiz `zord` com todos os subcomandos.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "zord",
		Short:         "CLI do ecossistema Zord: cria e gerencia projetos e suas ferramentas",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	for _, provide := range providers {
		root.AddCommand(provide()...)
	}
	return root
}
