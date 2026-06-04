// Package baseservice expõe os primitivos compartilhados entre todos os use
// cases (services): a struct base BaseService com helpers de erro legados,
// e as interfaces (ports) Logger, IdCreator e Validator que use cases
// recebem via injeção.
package baseservice

import (
	"net/http"
)

type Logger interface {
	Debug(Message string, Context ...string)
	Info(Message string, Context ...string)
	Warning(Message string, Context ...string)
	Error(Error error, Context ...string)
	Critical(Error error, Context ...string)
}

type IdCreator interface {
	Create() string
}

type Validator interface {
	ValidateStruct(modelData any) []error
}

// Error é o erro legado da BaseService (HTTP-aware). Novo código deve usar
// pkg/apperror.AppError; este tipo permanece pra manter compatibilidade com
// helpers existentes.
type Error struct {
	Status  int
	Message interface{}
	Error   string
}

type BaseService struct {
	Logger Logger
	Error  *Error
	Ulid   IdCreator
}

type Request interface {
}

func (bs *BaseService) errorHandler(httpStatus int, errMsg interface{}) {
	bs.Error = &Error{
		Status:  httpStatus,
		Message: errMsg,
		Error:   http.StatusText(httpStatus),
	}
}

func (bs *BaseService) CustomError(status int, err interface{}) {
	bs.errorHandler(status, err)
}

func (bs *BaseService) InternalServerError(msg string, err error) {
	bs.Logger.Error(err)
	bs.errorHandler(http.StatusInternalServerError, msg)
}

func (bs *BaseService) NotFound(msg string) {
	bs.errorHandler(http.StatusNotFound, msg)
}

func (bs *BaseService) BadRequest(msg string) {
	bs.errorHandler(http.StatusBadRequest, msg)
}

func (bs *BaseService) UnprocessableEntity(msg string) {
	bs.errorHandler(http.StatusUnprocessableEntity, msg)
}
