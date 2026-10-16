//go:build integration

// Testes de integração do BaseRepo contra um MySQL real. Não rodam no `go test`
// normal — exigem a tag `integration` e um DSN em ZORD_IT_DSN, ex.:
//
//	ZORD_IT_DSN='root:root@tcp(127.0.0.1:3307)/test?parseTime=true' \
//	  go test -tags integration ./internal/repositories/base_repository/ -v
package base_repository

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

// TestIntegration_EditCommits prova que o Edit persiste (executa e commita na
// própria tx) — o bug antigo executava fora da tx e commitava uma tx vazia.
func TestIntegration_EditCommits(t *testing.T) {
	ctx := context.Background()
	db := openIT(t)
	repo := NewBaseRepository[fakeDomain](db)

	tx, err := repo.InitTX(ctx)
	if err != nil {
		t.Fatalf("InitTX: %v", err)
	}
	if err := repo.Create(ctx, fakeDomain{ID: "01HFAKE000000000000000001", Name: "alice"}, tx, true); err != nil {
		t.Fatalf("Create: %v", err)
	}

	affected, err := repo.Edit(ctx, fakeDomain{ID: "01HFAKE000000000000000001", Name: "bob"}, "id", "01HFAKE000000000000000001")
	if err != nil {
		t.Fatalf("Edit: %v", err)
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

// TestIntegration_ValueIsNotInjectable prova que um valor malicioso no WHERE é
// tratado como literal parametrizado: não casa nenhuma linha e não afeta as
// outras (sem SQL injection).
func TestIntegration_ValueIsNotInjectable(t *testing.T) {
	ctx := context.Background()
	db := openIT(t)
	repo := NewBaseRepository[fakeDomain](db)

	tx, err := repo.InitTX(ctx)
	if err != nil {
		t.Fatalf("InitTX: %v", err)
	}
	if err := repo.Create(ctx, fakeDomain{ID: "keep-1", Name: "keep"}, tx, true); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Payload clássico: se o valor fosse interpolado, o WHERE viraria sempre
	// verdadeiro e o UPDATE atingiria todas as linhas.
	inject := "nope' OR '1'='1"
	affected, err := repo.Edit(ctx, fakeDomain{ID: "x", Name: "hacked"}, "id", inject)
	if err != nil {
		t.Fatalf("Edit: %v", err)
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

	tx, err := repo.InitTX(ctx)
	if err != nil {
		t.Fatalf("InitTX: %v", err)
	}
	if err := repo.Create(ctx, fakeDomain{ID: "del-1", Name: "x"}, tx, true); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, fakeDomain{}, "id", "del-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var deleted int
	if err := db.Get(&deleted, "SELECT COUNT(1) FROM fakes WHERE id = ? AND deleted_at IS NOT NULL", "del-1"); err != nil {
		t.Fatalf("check: %v", err)
	}
	if deleted != 1 {
		t.Errorf("soft delete não setou deleted_at (count = %d)", deleted)
	}
}
