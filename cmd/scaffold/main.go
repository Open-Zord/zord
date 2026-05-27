// Command scaffold é o gerador de código backend AST-puro (ver tools/scaffold).
package main

import (
	"fmt"
	"os"

	"github.com/Open-Zord/zord/tools/scaffold"
)

func main() {
	if err := scaffold.NewRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "scaffold:", err)
		os.Exit(1)
	}
}
