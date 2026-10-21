//go:build integration

// Testes de integração do BaseRepo contra um MySQL real. Não rodam no `go test`
// normal — exigem a tag `integration` e um DSN em ZORD_IT_DSN, ex.:
//
//	ZORD_IT_DSN='root:root@tcp(127.0.0.1:3307)/test?parseTime=true' \
//	  go test -tags integration ./internal/repositories/baserepository/ -v
package baserepository

import (
	"context"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func openIT(t *testing.T) *sqlx.DB {
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
	schema := []string{
		"DROP TABLE IF EXISTS fakes",
		"CREATE TABLE fakes (id CHAR(26) PRIMARY KEY, name VARCHAR(255) NOT NULL, deleted_at DATETIME NULL)",
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}
	return db
}

// seedCreate insere uma linha numa tx própria (begin → create → commit),
// exercitando a UnitOfWork.
func seedCreate(t *testing.T, ctx context.Context, uow *UoW, repo *BaseRepo[fakeDomain], d fakeDomain) {
	t.Helper()
	tx, err := uow.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := repo.Create(ctx, tx, d); err != nil {
		_ = tx.Rollback()
		t.Fatalf("Create: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

// TestIntegration_EditCommits prova que o Edit persiste quando o orquestrador
// commita a tx da UnitOfWork.
func TestIntegration_EditCommits(t *testing.T) {
	ctx := context.Background()
	db := openIT(t)
	repo := NewBaseRepository[fakeDomain](db)
	uow := NewUoW(db)

	seedCreate(t, ctx, uow, repo, fakeDomain{ID: "01HFAKE000000000000000001", Name: "alice"})

	tx, err := uow.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	affected, err := repo.Edit(ctx, tx, fakeDomain{ID: "01HFAKE000000000000000001", Name: "bob"}, "id", "01HFAKE000000000000000001")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("Edit: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if affected != 1 {
		t.Fatalf("Edit affected = %d; want 1", affected)
	}

	got, err := repo.Get(ctx, fakeDomain{}, "id", "01HFAKE000000000000000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "bob" {
		t.Errorf("name após Edit = %q; want bob (update não persistiu)", got.Name)
	}
}

// TestIntegration_RollbackDiscards prova que, sem commit, o Rollback descarta a
// escrita — o orquestrador controla o desfecho da tx.
func TestIntegration_RollbackDiscards(t *testing.T) {
	ctx := context.Background()
	db := openIT(t)
	repo := NewBaseRepository[fakeDomain](db)
	uow := NewUoW(db)

	seedCreate(t, ctx, uow, repo, fakeDomain{ID: "roll-1", Name: "orig"})

	tx, err := uow.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := repo.Edit(ctx, tx, fakeDomain{ID: "roll-1", Name: "mudado"}, "id", "roll-1"); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	got, err := repo.Get(ctx, fakeDomain{}, "id", "roll-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "orig" {
		t.Errorf("name após Rollback = %q; want orig (edit não devia persistir)", got.Name)
	}
}

// TestIntegration_ValueIsNotInjectable prova que um valor malicioso no WHERE é
// tratado como literal parametrizado: não casa nenhuma linha e não afeta as
// outras (sem SQL injection).
func TestIntegration_ValueIsNotInjectable(t *testing.T) {
	ctx := context.Background()
	db := openIT(t)
	repo := NewBaseRepository[fakeDomain](db)
	uow := NewUoW(db)

	seedCreate(t, ctx, uow, repo, fakeDomain{ID: "keep-1", Name: "keep"})

	// Payload clássico: se o valor fosse interpolado, o WHERE viraria sempre
	// verdadeiro e o UPDATE atingiria todas as linhas.
	inject := "nope' OR '1'='1"
	tx, err := uow.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	affected, err := repo.Edit(ctx, tx, fakeDomain{ID: "x", Name: "hacked"}, "id", inject)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("Edit: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if affected != 0 {
		t.Fatalf("Edit com payload afetou %d linhas; want 0 (valor deve ser literal)", affected)
	}

	// A linha legítima permanece intacta.
	got, err := repo.Get(ctx, fakeDomain{}, "id", "keep-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "keep" {
		t.Errorf("linha legítima foi alterada: name = %q; want keep", got.Name)
	}
}

// TestIntegration_SoftDelete prova que o soft delete seta deleted_at com o valor
// parametrizado e reporta a linha afetada.
func TestIntegration_SoftDelete(t *testing.T) {
	ctx := context.Background()
	db := openIT(t)
	repo := NewBaseRepository[fakeDomain](db)
	uow := NewUoW(db)

	seedCreate(t, ctx, uow, repo, fakeDomain{ID: "del-1", Name: "x"})

	tx, err := uow.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := repo.Delete(ctx, tx, fakeDomain{}, "id", "del-1"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("Delete: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var deleted int
	if err := db.Get(&deleted, "SELECT COUNT(1) FROM fakes WHERE id = ? AND deleted_at IS NOT NULL", "del-1"); err != nil {
		t.Fatalf("check: %v", err)
	}
	if deleted != 1 {
		t.Errorf("soft delete não setou deleted_at (count = %d)", deleted)
	}
}
