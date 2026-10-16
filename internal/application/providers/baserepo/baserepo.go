// Package baserepo expõe o contrato genérico de repositório de domínio.
// A interface Repository[T] é o conjunto canônico de operações que qualquer
// adapter de persistência precisa oferecer; o struct concreto BaseRepo[T]
// que a satisfaz vive em internal/repositories/base_repository. Convive em
// providers/ com baseservice (BaseService + ports Logger/IdCreator/Validator)
// como primitivo cross-cutting de aplicação.
package baserepo

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// Repository é o contrato genérico de persistência por domínio. Adapters
// concretos (ex.: BaseRepo em internal/repositories/base_repository) o
// implementam estruturalmente. Toda operação de I/O recebe context.Context
// para propagar cancelamento/deadline/tracing até o banco.
type Repository[dom Domain] interface {
	Get(ctx context.Context, domain dom, field string, value string) (*dom, error)
	Create(ctx context.Context, data dom, tx *sqlx.Tx, autoCommit bool) error
	List(ctx context.Context, domain dom, limit int, offset int) (*[]dom, error)
	Search(ctx context.Context, domain dom, field string, value string) (*[]dom, error)
	Edit(ctx context.Context, data dom, field string, value string) (int, error)
	Delete(ctx context.Context, domain dom, field string, values string) error
	Count(ctx context.Context, data dom) (int64, error)
	InitTX(ctx context.Context) (*sqlx.Tx, error)
	Commit(tx *sqlx.Tx) error
	Rollback(tx *sqlx.Tx, err error) error
}

// Domain é o conjunto mínimo de métodos que toda struct de domínio precisa
// expor pra ser persistível pelo Repository.
type Domain interface {
	Schema() string
	SoftDelete() string
}
