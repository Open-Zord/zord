// Command zord é o CLI do ecossistema Zord. Fase 1a: só `zord new`.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "zord",
		Short:         "CLI do ecossistema Zord",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newCommand())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "[ERRO]", err)
		os.Exit(1)
	}
}
