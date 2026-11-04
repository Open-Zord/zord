//go:build integration

// Teste de integração do use case List contra um MySQL real (tag `integration`
// + ZORD_IT_DSN).
package list

import (
	"context"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

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
		"INSERT INTO dummys (id, name, email) VALUES ('01HLIST000000000000000001', 'Ada', 'ada@example.com')",
		"INSERT INTO dummys (id, name, email) VALUES ('01HLIST000000000000000002', 'Linus', 'linus@example.com')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}
	return db
}

func TestIntegration_List(t *testing.T) {
	db := setupIT(t)
	svc := NewService(nil, nil, dummyrepo.NewDummyRepository(db))

	resp, err := svc.Execute(context.Background(), NewRequest(&Data{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(Items) = %d; want 2", len(resp.Items))
	}
}

func TestIntegration_ListPagination(t *testing.T) {
	db := setupIT(t)
	svc := NewService(nil, nil, dummyrepo.NewDummyRepository(db))

	resp, err := svc.Execute(context.Background(), NewRequest(&Data{Limit: 1, Offset: 0}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("len(Items) = %d; want 1 (limit=1)", len(resp.Items))
	}
}
