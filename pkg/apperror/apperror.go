// Package apperror define o erro de aplicação agnóstico de transporte.
// Carrega apenas uma categoria semântica (kind) e uma mensagem — nunca um HTTP
// status nem um code de API. O kind determina o HTTP status no transporte
// (cmd/http).
//
// AppError satisfaz a interface error: Error() devolve a mensagem. Os
// construtores padronizados são o único caminho de criação — o kind é
// privado, então não há var global nem literal arbitrário de kind.
package apperror

type AppError struct {
	kind    string
	Message string
	cause   error
}

// Error satisfaz a interface error. Quando há causa anexada, a cadeia é
// incluída (estilo fmt.Errorf("%w")), útil em logs. O transporte (cmd/http)
// não usa Error() para a resposta: usa Message nos erros de domínio e uma
// mensagem genérica nos erros internos — a causa nunca vaza na API.
func (e *AppError) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

// Kind devolve a categoria semântica do erro, consumida pelo transporte para
// mapear o HTTP status.
func (e *AppError) Kind() string { return e.kind }

// Unwrap expõe a causa anexada para errors.Is/errors.As.
func (e *AppError) Unwrap() error { return e.cause }

// WithCause anexa o erro de causa (consumido por errors.Is/As e pelos logs do
// transporte), preservando o kind e a Message voltada ao cliente. Encadeável:
//
//	return apperror.NewInternal("falha ao salvar").WithCause(err)
func (e *AppError) WithCause(cause error) *AppError {
	e.cause = cause
	return e
}

// Kinds semânticos. Privados ao pacote: o único caminho de criação são os
// construtores abaixo.
const (
	kindNotFound     = "not_found"
	kindConflict     = "conflict"
	kindInvalid      = "invalid"
	kindUnauthorized = "unauthorized"
	kindForbidden    = "forbidden"
	kindInternal     = "internal"
)

// NewNotFound constrói um AppError de recurso inexistente.
func NewNotFound(msg string) *AppError { return &AppError{kind: kindNotFound, Message: msg} }

// NewConflict constrói um AppError de conflito de estado (ex.: duplicidade).
func NewConflict(msg string) *AppError { return &AppError{kind: kindConflict, Message: msg} }

// NewInvalid constrói um AppError de entrada inválida (validação).
func NewInvalid(msg string) *AppError { return &AppError{kind: kindInvalid, Message: msg} }

// NewUnauthorized constrói um AppError de credencial ausente ou inválida.
func NewUnauthorized(msg string) *AppError {
	return &AppError{kind: kindUnauthorized, Message: msg}
}

// NewForbidden constrói um AppError de acesso negado a recurso existente.
func NewForbidden(msg string) *AppError { return &AppError{kind: kindForbidden, Message: msg} }

// NewInternal constrói um AppError de falha interna inesperada.
func NewInternal(msg string) *AppError { return &AppError{kind: kindInternal, Message: msg} }
