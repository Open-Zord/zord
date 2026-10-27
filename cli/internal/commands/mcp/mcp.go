// Package mcp expõe o servidor Model Context Protocol como `zord mcp`.
package mcp

import (
	"os"
	"os/signal"
	"syscall"

	mcptool "github.com/Open-Zord/zord/cli/tools/mcp"

	"github.com/spf13/cobra"
)

// Commands devolve o comando do servidor MCP para o root do CLI.
func Commands() []*cobra.Command {
	return []*cobra.Command{
		{
			Use:   "mcp [--repo <caminho>]",
			Short: "Sobe o servidor MCP (stdio) expondo scaffold e arch-analyse como tools",
			// O mcptool faz o próprio parse de --repo; repassamos os args crus.
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
				defer stop()
				return mcptool.Run(ctx, args)
			},
		},
	}
}
