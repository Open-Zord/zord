// Package migrate expõe os comandos de banco do CLI (`zord migrate`, `inspect`,
// `generate-schema-from-db`), que operam sobre o schema HCL do projeto atual.
package migrate

import (
	"fmt"
	"net/url"

	"github.com/Open-Zord/zord/cli/tools/migrator"
	"github.com/Open-Zord/zord/pkg/config"
	"github.com/Open-Zord/zord/pkg/logger"

	"github.com/spf13/cobra"
)

type runner struct {
	dsn      string
	dsnTest  string
	database string
}

// Commands devolve os comandos de banco para o root do CLI.
func Commands() []*cobra.Command {
	r := &runner{}
	return []*cobra.Command{
		{
			Use:    "migrate",
			Short:  "migrate HCL description database",
			PreRun: r.boot,
			RunE:   r.migrate,
		},
		{
			Use:    "inspect",
			Short:  "inspect HCL description database",
			PreRun: r.boot,
			RunE:   r.inspect,
		},
		{
			Use:    "generate-schema-from-db",
			Short:  "generate-schema-from-db <schema name> <database name>",
			Long:   "generate HCL schema from connection on env \ngenerate all databases if <database name> is not provided (all databases generations does`nt work with domain generation from schema)",
			PreRun: r.boot,
			RunE:   r.generate,
		},
	}
}

func (r *runner) migrate(_ *cobra.Command, args []string) error {
	tenant := ""
	if len(args) > 0 {
		tenant = args[0]
	}
	return migrator.NewMigrator(r.dsn, r.dsnTest, r.database).MigrateAllDomains(tenant)
}

func (r *runner) inspect(_ *cobra.Command, _ []string) error {
	return migrator.NewMigrator(r.dsn, r.dsnTest, r.database).Inspect()
}

func (r *runner) generate(_ *cobra.Command, args []string) error {
	schema := ""
	if len(args) > 0 {
		schema = args[0]
	}
	databaseName := ""
	if len(args) > 1 {
		databaseName = args[1]
	}
	return migrator.NewMigrator(r.dsn, r.dsnTest, r.database).Generate(schema, databaseName)
}

func (r *runner) boot(_ *cobra.Command, _ []string) {
	conf := config.NewConfig()
	if err := conf.LoadEnvsForEntrypoint("cli"); err != nil {
		panic(err)
	}

	r.database = conf.ReadConfig("DB_DATABASE")
	r.dsn = fmt.Sprintf(
		"%s:%s@%s:%s",
		url.QueryEscape(conf.ReadConfig("DB_USER")),
		url.QueryEscape(conf.ReadConfig("DB_PASS")),
		url.QueryEscape(conf.ReadConfig("DB_URL")),
		conf.ReadConfig("DB_PORT"),
	)
	r.dsnTest = fmt.Sprintf(
		"%s:%s@%s:%s/%s",
		conf.ReadConfig("DB_USER"),
		conf.ReadConfig("DB_PASS"),
		conf.ReadConfig("DB_URL"),
		conf.ReadConfig("DB_PORT"),
		conf.ReadConfig("DB_TEST_DATABASE"),
	)

	logger.NewLogger(
		conf.ReadConfig("ENVIRONMENT"),
		conf.ReadConfig("APP"),
		conf.ReadConfig("VERSION"),
	).Boot()
}
