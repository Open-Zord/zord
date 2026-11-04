// Command zord é o CLI único do ecossistema Zord: cria projetos (new) e expõe
// as ferramentas (scaffold, arch-analyse, migrate, mcp) como subcomandos. Os
// comandos vivem em internal/commands.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/Open-Zord/zord/internal/commands"
)

// version é injetada no release via -ldflags "-X main.version=..." (GoReleaser).
// Vazia em `go install`/`go build` sem ldflags — resolveVersion() cai no
// BuildInfo nesse caso.
var version = ""

func main() {
	root := commands.NewRootCmd()
	root.Version = resolveVersion()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "[ERRO]", err)
		os.Exit(1)
	}
}

// resolveVersion devolve a versão do binário na ordem: (1) injetada por ldflags
// no release; (2) a versão do módulo quando instalado via `go install ...@vX`,
// lida do build info; (3) "dev" em builds locais do fonte.
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
