package baserepository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// Prova de não-injeção sem banco real: diferente dos testes de integração
// (//go:build integration + MySQL), estes rodam no `go test` normal.
//
// A estratégia é dupla e se reforça:
//   - QueryMatcherEqual: o SQL emitido é comparado por igualdade exata contra a
//     string esperada (livre de payload). Se o código interpolasse o `value` na
//     query, o SQL real divergiria e a expectativa falharia.
//   - WithArgs(inject): prova que o `value` malicioso chega como parâmetro
//     bound (?/named), não concatenado ao texto da query.
//
// inject carrega aspas e `;` justamente para quebrar qualquer interpolação ingênua.
const inject = "x'; DROP TABLE fakes; --"

// hardDomain tem SoftDelete vazio para exercitar o branch de DELETE físico.
type hardDomain struct {
	ID   string `db:"id"`
	Name string `db:"name"`
}

func (hardDomain) Schema() string     { return "hards" }
func (hardDomain) SoftDelete() string { return "" }

// newMockRepo monta um BaseRepo[fakeDomain] apoiado num *sqlx.DB respaldado por
// sqlmock com matcher de igualdade exata. driver "mysql" garante bindvar `?`.
func newMockRepo(t *testing.T) (*BaseRepo[fakeDomain], sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	sx := sqlx.NewDb(db, "mysql")
	repo := &BaseRepo[fakeDomain]{Mysql: sx, fields: []string{"id", "name"}}
	return repo, mock
}

// beginTx abre uma Tx sqlx sobre o mock e a embrulha no txWrapper interno, para
// exercitar os métodos mutadores (Create/Edit/Delete) exatamente como o service.
func beginTx(t *testing.T, repo *BaseRepo[fakeDomain], mock sqlmock.Sqlmock) txWrapper {
	t.Helper()
	mock.ExpectBegin()
	tx, err := repo.Mysql.Beginx()
	if err != nil {
		t.Fatalf("Beginx: %v", err)
	}
	return txWrapper{tx: tx}
}

func mustMeet(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas não satisfeitas: %v", err)
	}
}

func TestSQLMock_Get_ParameterizesValue(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery("SELECT `id`, `name` FROM `fakes` WHERE `id` = ?").
		WithArgs(inject).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("1", "ada"))

	got, err := repo.Get(context.Background(), fakeDomain{}, "id", inject)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "1" || got.Name != "ada" {
		t.Errorf("Get = %#v; want {1 ada}", got)
	}
	mustMeet(t, mock)
}

func TestSQLMock_List_ParameterizesLimitOffset(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery("SELECT `id`, `name` FROM `fakes` LIMIT ? OFFSET ?").
		WithArgs(10, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("1", "ada"))

	got, err := repo.List(context.Background(), fakeDomain{}, 10, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Errorf("List = %#v; want 1 linha {1 ada}", got)
	}
	mustMeet(t, mock)
}

func TestSQLMock_Search_ParameterizesValue(t *testing.T) {
	repo, mock := newMockRepo(t)
	// O valor malicioso vai bound e ainda embrulhado em %...% — nada no texto.
	mock.ExpectQuery("SELECT `id`, `name` FROM `fakes` WHERE `name` LIKE ? LIMIT 25").
		WithArgs("%" + inject + "%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("1", "ada"))

	got, err := repo.Search(context.Background(), fakeDomain{}, "name", inject)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Search = %#v; want 1 linha", got)
	}
	mustMeet(t, mock)
}

func TestSQLMock_Count_QuotesTable(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery("SELECT count(1) FROM `fakes`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(7)))

	got, err := repo.Count(context.Background(), fakeDomain{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 7 {
		t.Errorf("Count = %d; want 7", got)
	}
	mustMeet(t, mock)
}

func TestSQLMock_Create_NamedParams(t *testing.T) {
	repo, mock := newMockRepo(t)
	tx := beginTx(t, repo, mock)
	// NamedExec reescreve :id,:name em ? na ordem textual; valores viram args.
	mock.ExpectExec("INSERT INTO `fakes` (`id`, `name`) VALUES (?, ?)").
		WithArgs(inject, "ada").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Create(context.Background(), tx, fakeDomain{ID: inject, Name: "ada"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	mustMeet(t, mock)
}

func TestSQLMock_Edit_ParameterizesValuesAndWhere(t *testing.T) {
	repo, mock := newMockRepo(t)
	tx := beginTx(t, repo, mock)
	// SET de cada coluna + WHERE: todos os valores (inclusive o do WHERE) bound.
	mock.ExpectExec("UPDATE `fakes` SET `id` = ?, `name` = ? WHERE `id` = ?").
		WithArgs("1", inject, inject).
		WillReturnResult(sqlmock.NewResult(0, 1))

	n, err := repo.Edit(context.Background(), tx, fakeDomain{ID: "1", Name: inject}, "id", inject)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if n != 1 {
		t.Errorf("Edit affected = %d; want 1", n)
	}
	mustMeet(t, mock)
}

func TestSQLMock_Delete_SoftParameterizesValue(t *testing.T) {
	repo, mock := newMockRepo(t)
	tx := beginTx(t, repo, mock)
	// fakeDomain.SoftDelete() == "deleted_at" -> UPDATE ... SET deleted_at = NOW().
	mock.ExpectExec("UPDATE `fakes` SET `deleted_at` = NOW() WHERE `id` = ?").
		WithArgs(inject).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(context.Background(), tx, fakeDomain{}, "id", inject); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	mustMeet(t, mock)
}

func TestSQLMock_Delete_HardParameterizesValue(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	sx := sqlx.NewDb(db, "mysql")
	repo := &BaseRepo[hardDomain]{Mysql: sx, fields: []string{"id", "name"}}

	mock.ExpectBegin()
	rawTx, err := sx.Beginx()
	if err != nil {
		t.Fatalf("Beginx: %v", err)
	}
	// hardDomain.SoftDelete() == "" -> DELETE físico, valor bound.
	mock.ExpectExec("DELETE FROM `hards` WHERE `id` = ?").
		WithArgs(inject).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(context.Background(), txWrapper{tx: rawTx}, hardDomain{}, "id", inject); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas não satisfeitas: %v", err)
	}
}
