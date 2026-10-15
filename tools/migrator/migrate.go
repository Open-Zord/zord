package migrator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"ariga.io/atlas-go-sdk/atlasexec"
)

const schemaHCLPath = "schemas/schema.my.hcl"

type Migrator struct {
	dsn          string
	dsnTest      string
	databaseName string
}

func NewMigrator(dsn string, dsnTest string, databaseName string) *Migrator {
	return &Migrator{
		dsn:          dsn,
		dsnTest:      dsnTest,
		databaseName: databaseName,
	}
}

// client abre um working dir Atlas sobre o HCL do projeto e um client da CLI
// atlas. O chamador é responsável por fechar o working dir devolvido.
func (m *Migrator) client() (*atlasexec.WorkingDir, *atlasexec.Client, error) {
	workdir, err := atlasexec.NewWorkingDir(
		atlasexec.WithAtlasHCLPath(schemaHCLPath),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("carregar working dir (%s): %w", schemaHCLPath, err)
	}
	client, err := atlasexec.NewClient(workdir.Path(), "atlas")
	if err != nil {
		_ = workdir.Close()
		return nil, nil, fmt.Errorf("inicializar client atlas (binário 'atlas' no PATH?): %w", err)
	}
	return workdir, client, nil
}

// MigrateAllDomains aplica o schema HCL no banco. tenant vazio usa o database
// padrão; caso contrário mira o schema/database do tenant informado.
func (m *Migrator) MigrateAllDomains(tenant string) error {
	workdir, client, err := m.client()
	if err != nil {
		return err
	}
	defer func() { _ = workdir.Close() }()

	params := &atlasexec.SchemaApplyParams{
		DevURL: "mysql://" + m.dsnTest,
		To:     "file://" + workdir.Path(),
		URL:    "mysql://" + m.dsn,
		Vars: atlasexec.Vars{
			"database": m.databaseName,
		},
		Schema: []string{
			m.databaseName,
		},
	}

	if tenant != "" {
		params.Vars = atlasexec.Vars{
			"database": tenant,
		}
		params.Schema = []string{
			tenant,
		}
	}

	res, err := client.SchemaApply(context.Background(), params)
	if err != nil {
		return fmt.Errorf("aplicar migrations: %w", err)
	}
	fmt.Printf("Applied %d changes:\n", len(res.Changes.Applied))
	fmt.Println(strings.Join(res.Changes.Applied, "\n"))
	return nil
}

// Inspect imprime o schema atual do banco descrito pelo HCL.
func (m *Migrator) Inspect() error {
	workdir, client, err := m.client()
	if err != nil {
		return err
	}
	defer func() { _ = workdir.Close() }()

	res, err := client.SchemaInspect(context.Background(), &atlasexec.SchemaInspectParams{
		DevURL: "mysql://" + m.dsnTest,
		URL:    "mysql://" + m.dsn,
	})
	if err != nil {
		return fmt.Errorf("inspecionar schema: %w", err)
	}
	fmt.Println(res)
	return nil
}

// Generate inspeciona o banco e grava o HCL resultante em schemas/<schemaName>.my.hcl.
func (m *Migrator) Generate(schemaName string, database string) error {
	workdir, client, err := m.client()
	if err != nil {
		return err
	}
	defer func() { _ = workdir.Close() }()

	if database != "" {
		database = "/" + database
	}
	res, err := client.SchemaInspect(context.Background(), &atlasexec.SchemaInspectParams{
		DevURL: "mysql://" + m.dsnTest,
		URL:    "mysql://" + m.dsn + database,
	})
	if err != nil {
		return fmt.Errorf("inspecionar schema: %w", err)
	}
	out := "schemas/" + schemaName + ".my.hcl"
	if err := os.WriteFile(out, []byte(res), 0o600); err != nil {
		return fmt.Errorf("gravar %s: %w", out, err)
	}
	return nil
}
