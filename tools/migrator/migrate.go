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

	res, err := client.SchemaApply(context.Background(), m.applyParams(tenant, workdir.Path()))
	if err != nil {
		return fmt.Errorf("aplicar migrations: %w", err)
	}
	fmt.Printf("Applied %d changes:\n", len(res.Changes.Applied))
	fmt.Println(strings.Join(res.Changes.Applied, "\n"))
	return nil
}

// applyParams monta os parâmetros do SchemaApply. tenant vazio mira o database
// padrão; caso contrário, o schema/database do tenant. Unifica Vars e Schema
// num único `database` (antes eram montados com databaseName e reescritos
// inteiros quando tenant != "").
func (m *Migrator) applyParams(tenant, workdirPath string) *atlasexec.SchemaApplyParams {
	database := m.databaseName
	if tenant != "" {
		database = tenant
	}
	return &atlasexec.SchemaApplyParams{
		DevURL: "mysql://" + m.dsnTest,
		To:     "file://" + workdirPath,
		URL:    "mysql://" + m.dsn,
		Vars:   atlasexec.Vars{"database": database},
		Schema: []string{database},
	}
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
		URL:    inspectURL(m.dsn, ""),
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

	res, err := client.SchemaInspect(context.Background(), &atlasexec.SchemaInspectParams{
		DevURL: "mysql://" + m.dsnTest,
		URL:    inspectURL(m.dsn, database),
	})
	if err != nil {
		return fmt.Errorf("inspecionar schema: %w", err)
	}
	if _, err := writeSchema(schemaName, res); err != nil {
		return err
	}
	return nil
}

// inspectURL monta a URL mysql do banco a inspecionar. database vazio mira a
// conexão sem database específico; caso contrário, acrescenta "/<database>".
func inspectURL(dsn, database string) string {
	if database != "" {
		database = "/" + database
	}
	return "mysql://" + dsn + database
}

// writeSchema grava o HCL inspecionado em schemas/<schemaName>.my.hcl com
// permissão 0o600 e devolve o caminho relativo escrito.
func writeSchema(schemaName, hcl string) (string, error) {
	out := "schemas/" + schemaName + ".my.hcl"
	if err := os.WriteFile(out, []byte(hcl), 0o600); err != nil {
		return "", fmt.Errorf("gravar %s: %w", out, err)
	}
	return out, nil
}
