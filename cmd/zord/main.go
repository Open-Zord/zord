// Command zord é o CLI único do ecossistema Zord: cria projetos (new) e expõe
// as ferramentas (scaffold, arch-analyse, migrate, mcp) como subcomandos. Os
// comandos vivem em internal/commands.
package main

import (
	"fmt"
	"os"

	"github.com/Open-Zord/zord/internal/commands"
)

func main() {
	if err := commands.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "[ERRO]", err)
		os.Exit(1)
	}
}
