package migrator

import (
	"fmt"
	"github.com/Open-Zord/zord/pkg/config"
	"github.com/Open-Zord/zord/pkg/logger"
	"github.com/Open-Zord/zord/tools/migrator"
	"net/url"

	"github.com/spf13/cobra"
)

type Migrator struct {
	dsn      string
	dsnTest  string
	database string
}

func NewMigrator() *Migrator {
	return &Migrator{}
}

func (m *Migrator) DeclareCommands(cmd *cobra.Command) {
	cmd.AddCommand(
		&cobra.Command{
			Use:    "migrate",
			Short:  "migrate HCL description database",
			PreRun: m.BootMigrator,
			RunE:   m.Migrate,
		},
		&cobra.Command{
			Use:    "inspect",
			Short:  "inspect HCL description database",
			PreRun: m.BootMigrator,
			RunE:   m.Inspect,
		},
		&cobra.Command{
			Use:    "generate-schema-from-db",
			Short:  "generate-schema-from-db <schema name> <database name>",
			Long:   "generate HCL schema from connection on env \ngenerate all databases if <database name> is not provided (all databases generations does`nt work with domain generation from schema)",
			PreRun: m.BootMigrator,
			RunE:   m.Generate,
		},
	)
}

func (m *Migrator) Migrate(_ *cobra.Command, args []string) error {
	tenant := ""
	if len(args) > 0 {
		tenant = args[0]
	}
	return migrator.NewMigrator(m.dsn, m.dsnTest, m.database).MigrateAllDomains(tenant)
}

func (m *Migrator) Inspect(_ *cobra.Command, _ []string) error {
	return migrator.NewMigrator(m.dsn, m.dsnTest, m.database).Inspect()
}

func (m *Migrator) Generate(_ *cobra.Command, args []string) error {
	schema := ""
	if len(args) > 0 {
		schema = args[0]
	}
	databaseName := ""
	if len(args) > 1 {
		databaseName = args[1]
	}
	return migrator.NewMigrator(m.dsn, m.dsnTest, m.database).Generate(schema, databaseName)
}

func (m *Migrator) BootMigrator(_ *cobra.Command, _ []string) {
	conf := config.NewConfig()
	err := conf.LoadEnvsForEntrypoint("cli")
	if err != nil {
		panic(err)
	}

	m.database = conf.ReadConfig("DB_DATABASE")
	dsn := "%s:%s@%s:%s"
	m.dsn = fmt.Sprintf(
		dsn,
		url.QueryEscape(conf.ReadConfig("DB_USER")),
		url.QueryEscape(conf.ReadConfig("DB_PASS")),
		url.QueryEscape(conf.ReadConfig("DB_URL")),
		conf.ReadConfig("DB_PORT"),
	)

	testDsn := "%s:%s@%s:%s/%s"
	m.dsnTest = fmt.Sprintf(
		testDsn,
		conf.ReadConfig("DB_USER"),
		conf.ReadConfig("DB_PASS"),
		conf.ReadConfig("DB_URL"),
		conf.ReadConfig("DB_PORT"),
		conf.ReadConfig("DB_TEST_DATABASE"),
	)

	l := logger.NewLogger(
		conf.ReadConfig("ENVIRONMENT"),
		conf.ReadConfig("APP"),
		conf.ReadConfig("VERSION"),
	)

	l.Boot()
}
