// Package httperr centraliza a serialização padrão de erros HTTP no formato
// {"code": string, "message": string} e concentra o mapeamento kind→status
// dos erros de aplicação (services.AppError). É o único ponto do transporte
// que conhece HTTP status; a camada de aplicação fala apenas semântica (kind).
package httperr

import (
	"net/http"
	"strings"

	"github.com/Open-Zord/zord/internal/application/services"

	"github.com/labstack/echo/v4"
)

// ErrorResponse é o payload padrão de erro emitido pela API.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// statusFor mapeia o kind semântico de um services.AppError para o HTTP status
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

// Respond serializa um error de service para a resposta JSON padrão. Quando o
// erro é um *services.AppError, o status vem do kind e o code é o genérico
// derivado dele (strings.ToUpper(kind)). Qualquer outro error cai no fallback
// INTERNAL/500.
func Respond(c echo.Context, err error) error {
	//nolint:errorlint // services retornam *AppError diretamente, sem wrapping %w
	if ae, ok := err.(*services.AppError); ok {
		return c.JSON(statusFor(ae.Kind()), ErrorResponse{
			Code:    strings.ToUpper(ae.Kind()),
			Message: ae.Message,
		})
	}
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
