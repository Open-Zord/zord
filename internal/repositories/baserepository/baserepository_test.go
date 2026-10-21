package baserepository

import (
	"context"
	"strings"
	"testing"
)

// fakeDomain é um domínio mínimo para exercitar o BaseRepo sem banco.
type fakeDomain struct {
	ID   string `db:"id"`
	Name string `db:"name"`
}

func (fakeDomain) Schema() string     { return "fakes" }
func (fakeDomain) SoftDelete() string { return "deleted_at" }

// taggedDomain tem um campo exportado SEM tag `db` (NoTag) para exercitar o
// filtro de tags vazias no construtor.
type taggedDomain struct {
	ID    string `db:"id"`
	NoTag string // exportado, sem db -> não vira coluna
	Name  string `db:"name"`
}

func (taggedDomain) Schema() string     { return "tagged" }
func (taggedDomain) SoftDelete() string { return "" }

// intDomain satisfaz Domain mas NÃO é struct — usado para o guard do construtor.
type intDomain int

func (intDomain) Schema() string     { return "ints" }
func (intDomain) SoftDelete() string { return "" }

// newFakeRepo constrói um BaseRepo com colunas conhecidas e sem *sqlx.DB — os
// caminhos testados aqui rejeitam antes de qualquer acesso ao banco.
func newFakeRepo() *BaseRepo[fakeDomain] {
	return &BaseRepo[fakeDomain]{fields: []string{"id", "name"}}
}

func TestQuoteIdent(t *testing.T) {
	cases := map[string]string{
		"id":        "`id`",
		"name":      "`name`",
		"a`b":       "`a``b`", // crase interna é escapada (duplicada)
		"x`;DROP--": "`x``;DROP--`",
	}
	for in, want := range cases {
		if got := quoteIdent(in); got != want {
			t.Errorf("quoteIdent(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestColumnAllowed(t *testing.T) {
	repo := newFakeRepo()
	for _, ok := range []string{"id", "name"} {
		if !repo.columnAllowed(ok) {
			t.Errorf("columnAllowed(%q) = false; want true", ok)
		}
	}
	for _, bad := range []string{"", "evil", "id; DROP TABLE fakes", "name) OR 1=1"} {
		if repo.columnAllowed(bad) {
			t.Errorf("columnAllowed(%q) = true; want false", bad)
		}
	}
}

// TestRejectsUnknownColumn garante que Get/Edit/Delete/Search recusam um field
// fora do conjunto conhecido ANTES de tocar o banco (defesa contra injeção por
// nome de coluna). Mysql é nil de propósito: se algum método não rejeitasse
// primeiro, haveria panic por nil-deref — então o teste também prova a ordem.
func TestRejectsUnknownColumn(t *testing.T) {
	repo := newFakeRepo()
	ctx := context.Background()
	d := fakeDomain{ID: "1", Name: "n"}
	const inject = "id; DROP TABLE fakes --"

	if _, err := repo.Get(ctx, d, inject, "x"); err == nil || !strings.Contains(err.Error(), "coluna desconhecida") {
		t.Errorf("Get: err = %v; want 'coluna desconhecida'", err)
	}
	// tx nil: a rejeição por coluna desconhecida acontece antes de tocar a tx.
	if _, err := repo.Edit(ctx, nil, d, inject, "x"); err == nil || !strings.Contains(err.Error(), "coluna desconhecida") {
		t.Errorf("Edit: err = %v; want 'coluna desconhecida'", err)
	}
	if err := repo.Delete(ctx, nil, d, inject, "x"); err == nil || !strings.Contains(err.Error(), "coluna desconhecida") {
		t.Errorf("Delete: err = %v; want 'coluna desconhecida'", err)
	}
	if _, err := repo.Search(ctx, d, inject, "x"); err == nil || !strings.Contains(err.Error(), "coluna desconhecida") {
		t.Errorf("Search: err = %v; want 'coluna desconhecida'", err)
	}
}

// TestNewBaseRepository_IgnoresFieldsWithoutDBTag garante que o construtor não
// transforma um campo exportado sem tag `db` numa coluna vazia (item #8).
func TestNewBaseRepository_IgnoresFieldsWithoutDBTag(t *testing.T) {
	repo := NewBaseRepository[taggedDomain](nil)
	want := []string{"id", "name"}
	if len(repo.fields) != len(want) {
		t.Fatalf("fields = %v; want %v", repo.fields, want)
	}
	for i := range want {
		if repo.fields[i] != want[i] {
			t.Errorf("fields[%d] = %q; want %q", i, repo.fields[i], want[i])
		}
	}
}

// TestNewBaseRepository_PanicsOnNonStruct garante o fail-fast claro quando o
// domínio satisfaz Domain mas não é struct (reflexão quebraria mais adiante).
func TestNewBaseRepository_PanicsOnNonStruct(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("esperava panic para domínio não-struct")
		}
	}()
	_ = NewBaseRepository[intDomain](nil)
}

func TestColumnValues(t *testing.T) {
	repo := newFakeRepo()
	got := repo.columnValues(&fakeDomain{ID: "1", Name: "ada"})
	if len(got) != 2 || got["id"] != "1" || got["name"] != "ada" {
		t.Errorf("columnValues = %#v; want {id:1, name:ada}", got)
	}
}
