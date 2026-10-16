package base_repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/structs"
	"github.com/jmoiron/sqlx"

	"github.com/Open-Zord/zord/internal/application/providers/baserepo"
)

type BaseRepo[dom baserepo.Domain] struct {
	Mysql  *sqlx.DB
	fields []string
}

func NewBaseRepository[dom baserepo.Domain](mysql *sqlx.DB) *BaseRepo[dom] {
	do := new(dom) // *dom: ponteiro-para-struct é a forma esperada por structs.
	if !structs.IsStruct(do) {
		panic(fmt.Sprintf("base_repository: domínio %T não é struct", *do))
	}
	listFields := make([]string, 0)
	for _, field := range structs.Fields(do) {
		if !field.IsExported() {
			continue
		}
		tag := field.Tag("db")
		if tag == "" {
			// Campo exportado sem tag `db` não é coluna: ignora em vez de
			// injetar um identificador vazio ("``") nas queries.
			continue
		}
		listFields = append(listFields, tag)
	}

	return &BaseRepo[dom]{
		Mysql:  mysql,
		fields: listFields,
	}
}

// txWrapper adapta *sqlx.Tx ao baserepo.Tx abstrato.
type txWrapper struct{ tx *sqlx.Tx }

func (t txWrapper) Commit() error   { return t.tx.Commit() }
func (t txWrapper) Rollback() error { return t.tx.Rollback() }

// UoW implementa baserepo.UnitOfWork sobre um *sqlx.DB.
type UoW struct {
	db *sqlx.DB
}

func NewUoW(db *sqlx.DB) *UoW {
	return &UoW{db: db}
}

func (u *UoW) Begin(ctx context.Context) (baserepo.Tx, error) {
	tx, err := u.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return txWrapper{tx: tx}, nil
}

// sqlxTxFrom recupera a *sqlx.Tx concreta de um baserepo.Tx. Erro explícito se
// vier uma Tx de outra implementação (nunca deve acontecer em runtime).
func sqlxTxFrom(tx baserepo.Tx) (*sqlx.Tx, error) {
	w, ok := tx.(txWrapper)
	if !ok {
		return nil, fmt.Errorf("base_repository: tx incompatível (%T)", tx)
	}
	return w.tx, nil
}

// quoteIdent cita um identificador SQL (tabela ou coluna) com crases, escapando
// crases internas. Identificadores não podem ir em placeholder ('?' só vale para
// valores), então a proteção contra injeção por nome é citar o identificador e
// — quando ele vem do chamador — validá-lo antes via columnAllowed.
func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// quoteIdents cita cada identificador da lista.
func quoteIdents(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = quoteIdent(n)
	}
	return out
}

// columnAllowed reporta se name é uma das colunas conhecidas do domínio
// (derivadas das tags `db`). É a defesa contra SQL injection por nome de coluna:
// como colunas não podem ser parametrizadas, o field vindo do chamador é
// rejeitado quando não pertence ao conjunto conhecido.
func (repo *BaseRepo[Domain]) columnAllowed(name string) bool {
	for _, f := range repo.fields {
		if f == name {
			return true
		}
	}
	return false
}

// columnValues extrai os valores do struct de domínio num map keyed pela tag
// `db`, para alimentar NamedExec sem interpolar valores na query.
func (repo *BaseRepo[Domain]) columnValues(data *Domain) map[string]any {
	out := make(map[string]any)
	for _, f := range structs.Fields(data) {
		if !f.IsExported() {
			continue
		}
		tag := f.Tag("db")
		if tag == "" {
			continue
		}
		out[tag] = f.Value()
	}
	return out
}

func (repo *BaseRepo[Domain]) Get(ctx context.Context, Data Domain, field string, value string) (*Domain, error) {
	if !repo.columnAllowed(field) {
		return nil, fmt.Errorf("get: coluna desconhecida %q", field)
	}
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = ?",
		strings.Join(quoteIdents(repo.fields), ", "),
		quoteIdent(Data.Schema()),
		quoteIdent(field),
	)
	row := repo.Mysql.QueryRowxContext(ctx, query, value)
	if err := row.StructScan(&Data); err != nil {
		return nil, err
	}
	return &Data, nil
}

// List devolve todas as linhas da tabela do Domain dentro do range
// LIMIT/OFFSET. Sem cláusula WHERE: filtros específicos viram métodos sob
// medida no repositório do domínio (não há mais filtro dinâmico genérico).
func (repo *BaseRepo[Domain]) List(ctx context.Context, Data Domain, limit int, offset int) (*[]Domain, error) {
	var results []Domain
	query := fmt.Sprintf(
		`SELECT %s FROM %s LIMIT ? OFFSET ?`,
		strings.Join(quoteIdents(repo.fields), ", "),
		quoteIdent(Data.Schema()),
	)
	rows, err := repo.Mysql.QueryxContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		if err := rows.StructScan(&Data); err != nil {
			return nil, err
		}
		results = append(results, Data)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &results, nil
}

// Count devolve a contagem total de linhas da tabela do Domain. Sem WHERE:
// counts filtrados viram métodos sob medida no repositório do domínio.
func (repo *BaseRepo[Domain]) Count(ctx context.Context, Data Domain) (int64, error) {
	var count int64
	err := repo.Mysql.GetContext(ctx, &count, "SELECT count(1) FROM "+quoteIdent(Data.Schema()))
	return count, err
}

func (repo *BaseRepo[Domain]) Search(ctx context.Context, Data Domain, field string, value string) (*[]Domain, error) {
	if !repo.columnAllowed(field) {
		return nil, fmt.Errorf("search: coluna desconhecida %q", field)
	}
	var data []Domain
	query := fmt.Sprintf(
		`SELECT %s FROM %s WHERE %s LIKE ? LIMIT 25`,
		strings.Join(quoteIdents(repo.fields), ", "),
		quoteIdent(Data.Schema()),
		quoteIdent(field),
	)
	if err := repo.Mysql.SelectContext(ctx, &data, query, "%"+value+"%"); err != nil {
		return nil, err
	}
	return &data, nil
}

// Create insere Data participando da transação tx. Não commita nem rola: o
// controle transacional é do orquestrador (service) que abriu a Tx.
func (repo *BaseRepo[Domain]) Create(ctx context.Context, tx baserepo.Tx, Data Domain) error {
	sx, err := sqlxTxFrom(tx)
	if err != nil {
		return err
	}
	namedValues := make([]string, 0, len(repo.fields))
	for _, field := range repo.fields {
		namedValues = append(namedValues, ":"+field)
	}
	res, err := sx.NamedExecContext(ctx,
		fmt.Sprintf(
			`INSERT INTO %s (%s) VALUES (%s)`,
			quoteIdent(Data.Schema()),
			strings.Join(quoteIdents(repo.fields), ", "),
			strings.Join(namedValues, ", "),
		),
		Data,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected < 1 {
		return errors.New("error on create, rows not affected")
	}
	return nil
}

// Edit atualiza todas as colunas de Data na linha onde field = value,
// participando da transação tx. Não commita nem rola.
func (repo *BaseRepo[Domain]) Edit(ctx context.Context, tx baserepo.Tx, Data Domain, field string, value string) (int, error) {
	if !repo.columnAllowed(field) {
		return 0, fmt.Errorf("edit: coluna desconhecida %q", field)
	}
	sx, err := sqlxTxFrom(tx)
	if err != nil {
		return 0, err
	}

	// Chave do valor de WHERE no map de named params. Prefixo improvável de
	// colidir com uma tag `db` real do domínio.
	const whereKey = "zord_where_value"
	setClauses := make([]string, 0, len(repo.fields))
	for _, f := range repo.fields {
		setClauses = append(setClauses, quoteIdent(f)+" = :"+f)
	}
	args := repo.columnValues(&Data)
	args[whereKey] = value
	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = :%s",
		quoteIdent(Data.Schema()),
		strings.Join(setClauses, ", "),
		quoteIdent(field),
		whereKey,
	)

	res, err := sx.NamedExecContext(ctx, query, args)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return int(affected), err
	}
	return int(affected), nil
}

// Delete remove (ou soft-deleta) a linha onde field = value, participando da
// transação tx. Não commita nem rola.
func (repo *BaseRepo[Domain]) Delete(ctx context.Context, tx baserepo.Tx, Data Domain, field string, value string) error {
	if !repo.columnAllowed(field) {
		return fmt.Errorf("delete: coluna desconhecida %q", field)
	}
	sx, err := sqlxTxFrom(tx)
	if err != nil {
		return err
	}

	var query string
	if softDelete := Data.SoftDelete(); softDelete != "" {
		query = fmt.Sprintf(
			"UPDATE %s SET %s = NOW() WHERE %s = ?",
			quoteIdent(Data.Schema()),
			quoteIdent(softDelete),
			quoteIdent(field),
		)
	} else {
		query = fmt.Sprintf(
			"DELETE FROM %s WHERE %s = ?",
			quoteIdent(Data.Schema()),
			quoteIdent(field),
		)
	}

	res, err := sx.ExecContext(ctx, query, value)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected < 1 {
		return errors.New("nothing deleted")
	}
	return nil
}
