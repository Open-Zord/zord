package services

// AppError é o erro de aplicação agnóstico de transporte. Carrega apenas uma
// categoria semântica (kind) e uma mensagem — nunca um HTTP status nem um
// code de API. O kind determina o HTTP status no transporte (cmd/http).
//
// AppError satisfaz a interface error: Error() devolve a mensagem. Os
// construtores padronizados são o único caminho de criação — o kind é
// privado, então não há var global nem literal arbitrário de kind.
type AppError struct {
	kind    string
	Message string
}

// Error satisfaz a interface error.
func (e *AppError) Error() string { return e.Message }

// Kind devolve a categoria semântica do erro, consumida pelo transporte para
// mapear o HTTP status.
func (e *AppError) Kind() string { return e.kind }

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
func NewUnauthorized(msg string) *AppError { return &AppError{kind: kindUnauthorized, Message: msg} }

// NewForbidden constrói um AppError de acesso negado a recurso existente.
func NewForbidden(msg string) *AppError { return &AppError{kind: kindForbidden, Message: msg} }

// NewInternal constrói um AppError de falha interna inesperada.
func NewInternal(msg string) *AppError { return &AppError{kind: kindInternal, Message: msg} }
