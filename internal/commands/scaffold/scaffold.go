// Package scaffold expõe o gerador de código como `zord scaffold ...`,
// reaproveitando a árvore de comandos de tools/scaffold.
package scaffold

import (
	scaffoldtool "github.com/Open-Zord/zord/internal/tools/scaffold"

	"github.com/spf13/cobra"
)

// Commands devolve a árvore do scaffold (já com Use: "scaffold") para o root.
func Commands() []*cobra.Command {
	return []*cobra.Command{scaffoldtool.NewRootCmd()}
}
