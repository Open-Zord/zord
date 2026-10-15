package main

import (
	"os"

	"github.com/Open-Zord/zord/cmd/cli/cli"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{}
	cliInstance := cli.NewCli(cmd)
	cliInstance.Start()
	if err := cliInstance.Cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
