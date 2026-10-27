package archanalyser

import (
	"strings"
	"testing"
)

// Caso válido: arquivo em internal/ importa stdlib, outro internal/ e a
// primitiva project-local pkg/apperror pelo module path DO PROJETO. Como o
// module path varia por projeto gerado, o gate strippa o prefixo do module
// antes de casar com a allow-list (entrada relativa "pkg/apperror").
func TestValidateImports_OK(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"pkg/apperror/apperror.go": `package apperror

type Error struct{}
`,
		"internal/application/domain/foo/foo.go": `package foo

func Hello() string { return "hi" }
`,
		"internal/application/services/foo/service.go": `package service

import (
	"fmt"

	"example.com/fake/pkg/apperror"
	dfoo "example.com/fake/internal/application/domain/foo"
)

func Use() (string, apperror.Error) { return fmt.Sprint(dfoo.Hello()), apperror.Error{} }
`,
	})
	if err := ValidateImports(root); err != nil {
		t.Fatalf("esperava sucesso, deu erro: %v", err)
	}
}

// Violação: arquivo em internal/ importa um pacote externo fora da allow-list.
func TestValidateImports_ExternoProibido(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/services/foo/service.go": `package service

import "github.com/evil/corp/malware"

func Use() { malware.Boom() }
`,
	})
	err := ValidateImports(root)
	if err == nil {
		t.Fatal("esperava erro para import externo proibido, mas passou")
	}
	if !strings.Contains(err.Error(), "malware") {
		t.Fatalf("erro não menciona o import infrator: %v", err)
	}
}
