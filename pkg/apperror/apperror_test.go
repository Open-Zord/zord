package apperror

import (
	"errors"
	"fmt"
	"testing"
)

func TestAppError_KindAndMessage(t *testing.T) {
	e := NewNotFound("recurso não existe")
	if e.Kind() != "not_found" {
		t.Errorf("Kind() = %q; want not_found", e.Kind())
	}
	if e.Error() != "recurso não existe" {
		t.Errorf("Error() = %q; want a mensagem crua", e.Error())
	}
	if e.Unwrap() != nil {
		t.Errorf("Unwrap() = %v; want nil (sem causa)", e.Unwrap())
	}
}

func TestAppError_WithCause(t *testing.T) {
	cause := errors.New("sql: no rows in result set")
	e := NewInternal("falha ao buscar").WithCause(cause)

	if !errors.Is(e, cause) {
		t.Error("errors.Is não encontrou a causa anexada")
	}
	if got, want := e.Error(), "falha ao buscar: sql: no rows in result set"; got != want {
		t.Errorf("Error() = %q; want %q", got, want)
	}
	// A mensagem voltada ao cliente permanece intacta.
	if e.Message != "falha ao buscar" {
		t.Errorf("Message = %q; want 'falha ao buscar'", e.Message)
	}
}

func TestAppError_ErrorsAsThroughWrap(t *testing.T) {
	base := NewConflict("duplicado")
	wrapped := fmt.Errorf("camada de serviço: %w", base)

	var ae *AppError
	if !errors.As(wrapped, &ae) {
		t.Fatal("errors.As não encontrou *AppError através do %w")
	}
	if ae.Kind() != "conflict" {
		t.Errorf("Kind() = %q; want conflict", ae.Kind())
	}
}
