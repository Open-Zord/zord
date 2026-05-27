package main

import (
	"github.com/spf13/cobra"
	"github.com/Open-Zord/zord/cmd/cli/cli"
)

func main() {
	cmd := &cobra.Command{}
	cliInstance := cli.NewCli(cmd)
	cliInstance.Start()
	cliInstance.Cmd.Execute()
}
