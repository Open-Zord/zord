// Command zord é o CLI único do ecossistema Zord: cria projetos (new) e expõe
// as ferramentas (scaffold, arch-analyse, migrate, mcp) como subcomandos. Os
// comandos vivem em internal/commands.
package main

import (
	"fmt"
	"os"

	"github.com/Open-Zord/zord/internal/commands"
)

// version é injetada no release via -ldflags "-X main.version=..." (GoReleaser).
// Em builds locais / `go install` sem ldflags, fica "dev".
var version = "dev"

func main() {
	root := commands.NewRootCmd()
	root.Version = version
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "[ERRO]", err)
		os.Exit(1)
	}
}
