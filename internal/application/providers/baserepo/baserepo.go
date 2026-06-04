// Package baserepo expõe o contrato genérico de repositório de domínio.
// A interface Repository[T] é o conjunto canônico de operações que qualquer
// adapter de persistência precisa oferecer; o struct concreto BaseRepo[T]
// que a satisfaz vive em internal/repositories/base_repository. Convive em
// providers/ com baseservice (BaseService + ports Logger/IdCreator/Validator)
// como primitivo cross-cutting de aplicação.
package baserepo

import (
	"github.com/jmoiron/sqlx"
)

// Repository é o contrato genérico de persistência por domínio. Adapters
// concretos (ex.: BaseRepo em internal/repositories/base_repository) o
// implementam estruturalmente.
type Repository[dom Domain] interface {
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

// Domain é o conjunto mínimo de métodos que toda struct de domínio precisa
// expor pra ser persistível pelo Repository.
type Domain interface {
	Schema() string
	SoftDelete() string
}
