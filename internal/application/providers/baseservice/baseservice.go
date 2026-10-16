// Package baseservice expõe os primitivos compartilhados entre todos os use
// cases (services): a struct base BaseService e as interfaces (ports) Logger,
// IdCreator e Validator que use cases recebem via injeção.
//
// Erros são sinalizados retornando pkg/apperror.AppError do Execute — não há
// mais estado de erro mutável no Service.
package baseservice

type Logger interface {
	Debug(message string, fields ...string)
	Info(message string, fields ...string)
	Warning(message string, fields ...string)
	Error(err error, fields ...string)
	Critical(err error, fields ...string)
}

type IdCreator interface {
	Create() string
}

type Validator interface {
	ValidateStruct(modelData any) error
}

type BaseService struct {
	Logger Logger
	Ulid   IdCreator
}
