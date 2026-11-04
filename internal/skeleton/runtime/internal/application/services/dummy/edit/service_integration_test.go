//go:build integration

// Teste de integração do use case Edit contra um MySQL real (tag `integration`
// + ZORD_IT_DSN).
package edit

import (
	"context"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/repositories/baserepository"
	dummyrepo "github.com/Open-Zord/zord/internal/skeleton/runtime/internal/repositories/dummy"
)

func setupIT(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("ZORD_IT_DSN")
	if dsn == "" {
		t.Skip("ZORD_IT_DSN não setado; pulando teste de integração")
	}
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS dummys",
		"CREATE TABLE dummys (id CHAR(26) PRIMARY KEY, name VARCHAR(255) NOT NULL, email VARCHAR(255) NOT NULL, deleted_at DATETIME NULL)",
		"INSERT INTO dummys (id, name, email) VALUES ('01HEDIT000000000000000001', 'Ada', 'ada@example.com')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}
	return db
}

func TestIntegration_Edit(t *testing.T) {
	db := setupIT(t)
	svc := NewService(nil, nil, dummyrepo.NewDummyRepository(db), baserepository.NewUoW(db))

	resp, err := svc.Execute(context.Background(), NewRequest(&Data{
		ID: "01HEDIT000000000000000001", Name: "Ada Lovelace", Email: "ada@novo.com",
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.Updated != 1 {
		t.Fatalf("Updated = %d; want 1", resp.Updated)
	}

	var got struct {
		Name  string `db:"name"`
		Email string `db:"email"`
	}
	if err := db.Get(&got, "SELECT name, email FROM dummys WHERE id = ?", "01HEDIT000000000000000001"); err != nil {
		t.Fatalf("reler: %v", err)
	}
	if got.Name != "Ada Lovelace" || got.Email != "ada@novo.com" {
		t.Fatalf("linha = %+v; want {Ada Lovelace, ada@novo.com}", got)
	}
}
