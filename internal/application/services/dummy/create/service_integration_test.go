//go:build integration

// Teste de integração do use case Create contra um MySQL real. Exige a tag
// `integration` e ZORD_IT_DSN, ex.:
//
//	ZORD_IT_DSN='root:root@tcp(127.0.0.1:3307)/test?parseTime=true' \
//	  go test -tags integration ./internal/application/services/dummy/create/ -v
package create

import (
	"context"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/Open-Zord/zord/internal/repositories/baserepository"
	dummyrepo "github.com/Open-Zord/zord/internal/repositories/dummy"
)

type nopLogger struct{}

func (nopLogger) Debug(string, ...string)   {}
func (nopLogger) Info(string, ...string)    {}
func (nopLogger) Warning(string, ...string) {}
func (nopLogger) Error(error, ...string)    {}
func (nopLogger) Critical(error, ...string) {}

type fixedID struct{}

func (fixedID) Create() string { return "01HSVCCREATE0000000000001" }

// TestIntegration_CreatePersists exercita o fluxo completo do use case: ctx ->
// UnitOfWork -> repo.Create -> commit, provando que o service de fato persiste
// (antes era stub e ignorava o banco).
func TestIntegration_CreatePersists(t *testing.T) {
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
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	repo := dummyrepo.NewDummyRepository(db)
	uow := baserepository.NewUoW(db)
	svc := NewService(nopLogger{}, fixedID{}, repo, uow)

	resp, err := svc.Execute(context.Background(), NewRequest(&Data{Name: "Ada", Email: "ada@example.com"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.ID != "01HSVCCREATE0000000000001" {
		t.Fatalf("Response.ID = %q; want o id gerado", resp.ID)
	}

	var got struct {
		Name  string `db:"name"`
		Email string `db:"email"`
	}
	if err := db.Get(&got, "SELECT name, email FROM dummys WHERE id = ?", resp.ID); err != nil {
		t.Fatalf("linha não persistida: %v", err)
	}
	if got.Name != "Ada" || got.Email != "ada@example.com" {
		t.Errorf("linha = %+v; want {Ada, ada@example.com}", got)
	}
}
