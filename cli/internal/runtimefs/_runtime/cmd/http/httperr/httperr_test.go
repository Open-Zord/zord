package httperr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Open-Zord/zord/pkg/apperror"

	"github.com/labstack/echo/v4"
)

// respond roda Respond num contexto Echo de teste e devolve o status e o corpo.
func respond(t *testing.T, err error) (int, ErrorResponse) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if rerr := Respond(c, err); rerr != nil {
		t.Fatalf("Respond devolveu erro: %v", rerr)
	}
	var body ErrorResponse
	if uerr := json.Unmarshal(rec.Body.Bytes(), &body); uerr != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), uerr)
	}
	return rec.Code, body
}

func TestRespond_MapsKindToStatus(t *testing.T) {
	code, body := respond(t, apperror.NewNotFound("x não existe"))
	if code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", code)
	}
	if body.Code != "NOT_FOUND" || body.Message != "x não existe" {
		t.Errorf("body = %+v; want {NOT_FOUND, x não existe}", body)
	}
}

// TestRespond_FindsWrappedAppError é o ganho central da troca para errors.As:
// um AppError embrulhado com %w continua mapeando pelo kind. Com a type
// assertion direta anterior, isto cairia no 500 genérico.
func TestRespond_FindsWrappedAppError(t *testing.T) {
	err := fmt.Errorf("camada de serviço: %w", apperror.NewConflict("duplicado"))
	code, body := respond(t, err)
	if code != http.StatusConflict {
		t.Errorf("status = %d; want 409", code)
	}
	if body.Code != "CONFLICT" {
		t.Errorf("code = %q; want CONFLICT", body.Code)
	}
}

// TestRespond_InternalDoesNotLeakMessage garante que erro interno não expõe o
// detalhe interno ao cliente.
func TestRespond_InternalDoesNotLeakMessage(t *testing.T) {
	code, body := respond(t, apperror.NewInternal("stacktrace secreto do banco"))
	if code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500", code)
	}
	if body.Code != "INTERNAL" || body.Message != "erro interno" {
		t.Errorf("body = %+v; want genérico {INTERNAL, erro interno}", body)
	}
}

func TestRespond_RawErrorIsGeneric(t *testing.T) {
	code, body := respond(t, errors.New("boom cru"))
	if code != http.StatusInternalServerError || body.Message != "erro interno" {
		t.Errorf("status=%d body=%+v; want 500 genérico", code, body)
	}
}
