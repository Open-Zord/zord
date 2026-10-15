package base_repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/structs"
	"github.com/jmoiron/sqlx"
)

type BaseRepository[dom Domain] interface {
	Get(domain dom, field string, value string) (*dom, error)
	Create(data dom, tx *sqlx.Tx, autoCommit bool) error
	List(domain dom, limit int, offset int) (*[]dom, error)
	Search(domain dom, field string, value string) (*[]dom, error)
	Edit(data dom, field string, value string) (int, error)
	Delete(domain dom, field string, values string) error
	Count(Data dom) (int64, error)
	InitTX() (*sqlx.Tx, error)
	Commit(tx *sqlx.Tx) error
	Rollback(tx *sqlx.Tx, err error) error
}

type Domain interface {
	Schema() string
	SoftDelete() string
}

type BaseRepo[dom Domain] struct {
	Mysql  *sqlx.DB
	fields []string
}

func NewBaseRepository[dom Domain](mysql *sqlx.DB) *BaseRepo[dom] {
	do := new(dom)
	listFields := []string{}
	fields := structs.Fields(&do)
	for _, field := range fields {
		if !field.IsExported() {
			continue
		}
		tag := field.Tag("db")
		listFields = append(listFields, tag)
	}

	return &BaseRepo[dom]{
		Mysql:  mysql,
		fields: listFields,
	}
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

func (repo *BaseRepo[Domain]) InitTX() (*sqlx.Tx, error) {
	tx, err := repo.Mysql.Beginx()
	return tx, err
}

func (repo *BaseRepo[Domain]) Commit(tx *sqlx.Tx) error {
	return tx.Commit()
}

func (repo *BaseRepo[Domain]) Rollback(tx *sqlx.Tx, err error) error {
	rollbErr := tx.Rollback()
	if rollbErr != nil {
		return rollbErr
	}
	return err
}

func (repo *BaseRepo[Domain]) Get(Data Domain, field string, value string) (*Domain, error) {
	if !repo.columnAllowed(field) {
		return nil, fmt.Errorf("get: coluna desconhecida %q", field)
	}
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = ?",
		strings.Join(quoteIdents(repo.fields), ", "),
		quoteIdent(Data.Schema()),
		quoteIdent(field),
	)
	row := repo.Mysql.QueryRowx(query, value)
	if err := row.StructScan(&Data); err != nil {
		return nil, err
	}
	return &Data, nil
}

func (repo *BaseRepo[Domain]) Create(Data Domain, tx *sqlx.Tx, autoCommit bool) error {
	namedValues := make([]string, 0, len(repo.fields))
	for _, field := range repo.fields {
		namedValues = append(namedValues, ":"+field)
	}
	res, err := tx.NamedExec(
		fmt.Sprintf(
			`INSERT INTO %s (%s) VALUES (%s)`,
			quoteIdent(Data.Schema()),
			strings.Join(quoteIdents(repo.fields), ", "),
			strings.Join(namedValues, ", "),
		),
		Data,
	)
	if err != nil {
		return repo.Rollback(tx, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return repo.Rollback(tx, err)
	}
	if affected < 1 {
		return repo.Rollback(tx, errors.New("error on create, rows not affected"))
	}

	if autoCommit {
		if err := tx.Commit(); err != nil {
			return repo.Rollback(tx, err)
		}
	}

	return nil
}

// List devolve todas as linhas da tabela do Domain dentro do range
// LIMIT/OFFSET. Sem cláusula WHERE: filtros específicos viram métodos sob
// medida no repositório do domínio (não há mais filtro dinâmico genérico).
func (repo *BaseRepo[Domain]) List(Data Domain, limit int, offset int) (*[]Domain, error) {
	var results []Domain
	query := fmt.Sprintf(
		`SELECT %s FROM %s LIMIT ? OFFSET ?`,
		strings.Join(quoteIdents(repo.fields), ", "),
		quoteIdent(Data.Schema()),
	)
	rows, err := repo.Mysql.Queryx(query, limit, offset)
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

func (repo *BaseRepo[Domain]) Edit(Data Domain, field string, value string) (int, error) {
	if !repo.columnAllowed(field) {
		return 0, fmt.Errorf("edit: coluna desconhecida %q", field)
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

	tx, err := repo.Mysql.Beginx()
	if err != nil {
		return 0, err
	}

	res, err := tx.NamedExec(query, args)
	if err != nil {
		return 0, repo.Rollback(tx, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return int(affected), repo.Rollback(tx, err)
	}
	if affected < 1 {
		return int(affected), repo.Rollback(tx, nil)
	}

	if err := repo.Commit(tx); err != nil {
		return int(affected), repo.Rollback(tx, err)
	}
	return int(affected), nil
}

func (repo *BaseRepo[Domain]) Delete(Data Domain, field string, value string) error {
	if !repo.columnAllowed(field) {
		return fmt.Errorf("delete: coluna desconhecida %q", field)
	}

	softDelete := Data.SoftDelete()
	if softDelete != "" {
		query := fmt.Sprintf(
			"UPDATE %s SET %s = NOW() WHERE %s = ?",
			quoteIdent(Data.Schema()),
			quoteIdent(softDelete),
			quoteIdent(field),
		)
		return repo.execDelete(query, value)
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = ?",
		quoteIdent(Data.Schema()),
		quoteIdent(field),
	)
	return repo.execDelete(query, value)
}

// execDelete roda um DELETE/soft-delete já montado (identificadores citados,
// valor parametrizado) e trata "nada afetado" como erro.
func (repo *BaseRepo[Domain]) execDelete(query string, value string) error {
	res, err := repo.Mysql.Exec(query, value)
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

// Count devolve a contagem total de linhas da tabela do Domain. Sem WHERE:
// counts filtrados viram métodos sob medida no repositório do domínio.
func (repo *BaseRepo[Domain]) Count(Data Domain) (int64, error) {
	var count int64
	err := repo.Mysql.Get(&count, "SELECT count(1) FROM "+quoteIdent(Data.Schema()))
	return count, err
}

func (repo *BaseRepo[Domain]) Search(Data Domain, field string, value string) (*[]Domain, error) {
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
	if err := repo.Mysql.Select(&data, query, "%"+value+"%"); err != nil {
		return nil, err
	}
	return &data, nil
}
