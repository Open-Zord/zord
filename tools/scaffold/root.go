// Package scaffold (área cmd) liga os subcomandos cobra expostos pelo binário scaffold.
package scaffold

import "github.com/spf13/cobra"

// NewRootCmd builds the scaffold command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "scaffold",
		Short:         "Gera código backend camada a camada (AST puro)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newDomainCmd())
	root.AddCommand(newFieldCmd())
	root.AddCommand(newDeriveCmd())
	root.AddCommand(newRepositoryCmd())
	root.AddCommand(newServiceCmd())
	root.AddCommand(newRequestCmd())
	root.AddCommand(newResponseCmd())
	root.AddCommand(newHandlerCmd())
	root.AddCommand(newRouteCmd())
	root.AddCommand(newProjectionCmd())
	return root
}
