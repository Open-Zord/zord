package archanalyser

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// Raiz ausente não é erro: nada a percorrer, fn não é chamada.
func TestWalkGoFiles_MissingRootIsNil(t *testing.T) {
	called := false
	err := walkGoFiles(filepath.Join(t.TempDir(), "nao-existe"), func(string) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("raiz ausente deveria retornar nil, got: %v", err)
	}
	if called {
		t.Error("fn não deveria ser chamada para raiz ausente")
	}
}

// Erro retornado por fn é propagado (não engolido).
func TestWalkGoFiles_PropagatesCallbackError(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/foo/foo.go": "package foo\n",
	})
	sentinel := errors.New("boom")
	err := walkGoFiles(filepath.Join(root, "internal"), func(string) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("esperado propagar o erro da fn, got: %v", err)
	}
}

// Regressão do item #15: um arquivo .go não-parseável em internal/ deixa de
// passar despercebido — o validador falha (fail-closed) em vez de pular em
// silêncio.
func TestValidateNoGlobalVars_UnparseableFileFails(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/foo/foo.go": "package foo\n\nfunc (((\n", // sintaxe inválida
	})
	err := ValidateNoGlobalVars(root)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("esperado falha de parse para arquivo ilegível, got: %v", err)
	}
}
