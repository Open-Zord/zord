// Package baseservice expõe os primitivos compartilhados entre todos os use
// cases (services): a struct base BaseService e as interfaces (ports) Logger,
// IdCreator e Validator que use cases recebem via injeção.
//
// Erros são sinalizados retornando pkg/apperror.AppError do Execute — não há
// mais estado de erro mutável no Service.
package baseservice

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

type BaseService struct {
	Logger Logger
	Ulid   IdCreator
}

type Request interface {
}
