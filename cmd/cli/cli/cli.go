package cli

import (
	"github.com/Open-Zord/zord/cmd/cli/archanalyser"
	"github.com/Open-Zord/zord/cmd/cli/migrator"

	"github.com/spf13/cobra"
)

type Cli struct {
	Cmd *cobra.Command
}

func NewCli(cmd *cobra.Command) *Cli {
	return &Cli{
		Cmd: cmd,
	}
}

func (c *Cli) Start() {
	migratorInstance := migrator.NewMigrator()
	migratorInstance.DeclareCommands(c.Cmd)
	archAnalyserInstance := archanalyser.NewArchAnalyser()
	archAnalyserInstance.DeclareCommands(c.Cmd)
}
