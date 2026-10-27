// Package httperr centraliza a serialização padrão de erros HTTP no formato
// {"code": string, "message": string} e concentra o mapeamento kind→status
// dos erros de aplicação (apperror.AppError). É o único ponto do transporte
// que conhece HTTP status; a camada de aplicação fala apenas semântica (kind).
package httperr

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/apperror"

	"github.com/labstack/echo/v4"
)

// ErrorResponse é o payload padrão de erro emitido pela API.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// statusFor mapeia o kind semântico de um apperror.AppError para o HTTP status
// equivalente. Qualquer kind desconhecido (ou "internal") cai em 500.
func statusFor(kind string) int {
	switch kind {
	case "not_found":
		return http.StatusNotFound
	case "conflict":
		return http.StatusConflict
	case "invalid":
		return http.StatusUnprocessableEntity
	case "unauthorized":
		return http.StatusUnauthorized
	case "forbidden":
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// Respond serializa um error de service para a resposta JSON padrão. Usa
// errors.As, então um *apperror.AppError embrulhado com %w continua sendo
// mapeado pelo kind. Erros de domínio (status < 500) devolvem o code derivado
// do kind e a Message. Erros internos (kind internal, kind desconhecido ou
// error cru) logam a causa via c.Logger() e devolvem uma resposta genérica,
// sem vazar detalhe interno na API.
func Respond(c echo.Context, err error) error {
	var ae *apperror.AppError
	if errors.As(err, &ae) {
		if status := statusFor(ae.Kind()); status < http.StatusInternalServerError {
			return c.JSON(status, ErrorResponse{
				Code:    strings.ToUpper(ae.Kind()),
				Message: ae.Message,
			})
		}
	}
	c.Logger().Error(err)
	return c.JSON(http.StatusInternalServerError, ErrorResponse{
		Code:    "INTERNAL",
		Message: "erro interno",
	})
}

// RespondBadRequest emite um 400 com code=BAD_REQUEST e a mensagem fornecida.
// Usado pelos handlers para erros de bind/parse do payload, anteriores ao
// service.
func RespondBadRequest(c echo.Context, message string) error {
	return c.JSON(http.StatusBadRequest, ErrorResponse{Code: "BAD_REQUEST", Message: message})
}
