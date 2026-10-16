// Package baserepo expõe o contrato genérico de repositório de domínio.
// A interface Repository[T] é o conjunto canônico de operações que qualquer
// adapter de persistência precisa oferecer; o struct concreto BaseRepo[T]
// que a satisfaz vive em internal/repositories/base_repository. Convive em
// providers/ com baseservice (BaseService + ports Logger/IdCreator/Validator)
// como primitivo cross-cutting de aplicação.
package baserepo

import "context"

// Tx é o handle de transação abstrato devolvido pela UnitOfWork. Esconde o
// driver concreto (sqlx) do contrato de aplicação: o service orquestra
// Begin/Commit/Rollback sem conhecer o driver.
type Tx interface {
	Commit() error
	Rollback() error
}

// UnitOfWork abre transações. Um service chama Begin para obter uma Tx, passa
// essa Tx às operações mutadoras do Repository e commita/rola ao final — o
// controle transacional fica no orquestrador, não no repositório.
type UnitOfWork interface {
	Begin(ctx context.Context) (Tx, error)
}

// Repository é o contrato genérico de persistência por domínio. Adapters
// concretos (ex.: BaseRepo em internal/repositories/base_repository) o
// implementam estruturalmente. Operações de leitura usam o pool; as mutadoras
// (Create/Edit/Delete) recebem uma Tx aberta pela UnitOfWork. Toda operação
// recebe context.Context para propagar cancelamento/deadline/tracing.
type Repository[dom Domain] interface {
	Get(ctx context.Context, domain dom, field string, value string) (*dom, error)
	List(ctx context.Context, domain dom, limit int, offset int) ([]dom, error)
	Search(ctx context.Context, domain dom, field string, value string) ([]dom, error)
	Count(ctx context.Context, data dom) (int64, error)
	Create(ctx context.Context, tx Tx, data dom) error
	Edit(ctx context.Context, tx Tx, data dom, field string, value string) (int, error)
	Delete(ctx context.Context, tx Tx, data dom, field string, value string) error
}

// Domain é o conjunto mínimo de métodos que toda struct de domínio precisa
// expor pra ser persistível pelo Repository.
type Domain interface {
	Schema() string
	SoftDelete() string
}
