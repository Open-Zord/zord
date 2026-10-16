package logger

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// RegistryKey is the key under which the logger is registered in the registry.
const RegistryKey = "logger"

type Logger struct {
	env          string
	app          string
	version      string
	logger       zerolog.Logger
	isProduction bool
	service      string
}

func NewLogger(environment, app, version string) *Logger {
	return &Logger{
		env:          environment,
		app:          app,
		version:      version,
		isProduction: environment == "production",
	}
}

func (l *Logger) Boot() {
	zerolog.CallerFieldName = "trace"
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMicro

	l.logger = log.With().
		Str("app", l.app).
		Str("version", l.version).
		Str("environment", l.env).
		Logger()
}

func (l *Logger) Debug(message string, fields ...string) {
	l.logger.Debug().Str("service", l.service).Strs("context", fields).Msg(message)
}

func (l *Logger) Info(message string, fields ...string) {
	l.logger.Info().
		Str("service", l.service).Strs("context", fields).Msg(message)
}

func (l *Logger) Warning(message string, fields ...string) {
	l.logger.Warn().
		Str("service", l.service).Strs("context", fields).Msg(message)
}

func (l *Logger) Error(err error, fields ...string) {
	l.logger.Error().Str("service", l.service).Strs(
		"context", fields).
		Caller(1).Msg(err.Error())
}

// Critical loga em nível fatal mas NÃO encerra o processo (diferente de
// zerolog.Fatal): a decisão de sair é de quem chama, no edge (ex.: o server).
func (l *Logger) Critical(err error, fields ...string) {
	l.logger.WithLevel(zerolog.FatalLevel).Str("service", l.service).Strs(
		"context", fields).
		Caller(1).Msg(err.Error())
}

func (l *Logger) SetLogService(service string) {
	l.service = service
}
