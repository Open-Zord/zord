package arch_analyser

import (
	"strings"
	"testing"
)

// --- ValidateContextUsage (AST) ---

// Import de context fora de services/repositories/baserepo é violação.
func TestValidateContextUsage_RejectsImportOutsideAllowed(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/domain/user/user.go": `package user

import "context"

func Use(ctx context.Context) {}
`,
	})
	err := ValidateContextUsage(root)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("esperado violação por import de context, got: %v", err)
	}
}

// Regressão: mencionar context.Context em comentário/string NÃO é violação
// (o antigo strings.Contains sobre texto bruto falsamente acusava).
func TestValidateContextUsage_IgnoresCommentAndString(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/domain/user/user.go": `package user

// Este domínio não usa context.Context diretamente.
const Doc = "veja context.Context na doc"
`,
	})
	if err := ValidateContextUsage(root); err != nil {
		t.Fatalf("esperado sem violação para menção textual, got: %v", err)
	}
}

// Services pode importar context.
func TestValidateContextUsage_AllowsInServices(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/services/user/create/service.go": `package create

import "context"

func Do(ctx context.Context) {}
`,
	})
	if err := ValidateContextUsage(root); err != nil {
		t.Fatalf("esperado sem violação em services, got: %v", err)
	}
}

// --- ValidateTypeAssertions (AST) ---

// Assertion de valor único é violação.
func TestValidateTypeAssertions_RejectsSingleValue(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/foo/foo.go": `package foo

func f(x any) string {
	return x.(string)
}
`,
	})
	err := ValidateTypeAssertions(root)
	if err == nil || !strings.Contains(err.Error(), "type assertion") {
		t.Fatalf("esperado violação para assertion de valor único, got: %v", err)
	}
}

// Vírgula-ok (:= e =) e type switch são permitidos.
func TestValidateTypeAssertions_AllowsCommaOkAndSwitch(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/foo/foo.go": `package foo

func f(x any) {
	if s, ok := x.(string); ok {
		_ = s
	}
	var y string
	var ok bool
	y, ok = x.(string)
	_, _ = y, ok
	switch x.(type) {
	case int:
	}
}
`,
	})
	if err := ValidateTypeAssertions(root); err != nil {
		t.Fatalf("esperado sem violação para vírgula-ok/type switch, got: %v", err)
	}
}

// Regressão: ".(" em comentário/string não conta como assertion.
func TestValidateTypeAssertions_IgnoresCommentAndString(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/foo/foo.go": `package foo

// exemplo: v := x.(T) é proibido
const Doc = "use v, ok := x.(T)"
`,
	})
	if err := ValidateTypeAssertions(root); err != nil {
		t.Fatalf("esperado sem violação para menção textual, got: %v", err)
	}
}

// --- ValidateDbQueriesInRepositories (AST) ---

// Chamada direta (bare) de função de query fora de repositories é violação.
func TestValidateDbQueries_RejectsBareCall(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/services/user/create/service.go": `package create

func Query(s string) {}

func Do() {
	Query("SELECT 1")
}
`,
	})
	err := ValidateDbQueriesInRepositories(root)
	if err == nil || !strings.Contains(err.Error(), "Query") {
		t.Fatalf("esperado violação para bare call Query, got: %v", err)
	}
}

// Chamada de método/pacote (db.Query) é permitida fora de repositories.
func TestValidateDbQueries_AllowsMethodCall(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/services/user/create/service.go": `package create

type db struct{}

func (db) Query(s string) {}

func Do(d db) {
	d.Query("SELECT 1")
}
`,
	})
	if err := ValidateDbQueriesInRepositories(root); err != nil {
		t.Fatalf("esperado sem violação para método, got: %v", err)
	}
}

// Dentro de repositories/, bare call é permitido (é a casa das queries).
func TestValidateDbQueries_AllowsInsideRepositories(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/repositories/user/user.go": `package userrepo

func Query(s string) {}

func Do() {
	Query("SELECT 1")
}
`,
	})
	if err := ValidateDbQueriesInRepositories(root); err != nil {
		t.Fatalf("esperado sem violação dentro de repositories, got: %v", err)
	}
}

// Regressão: nome de query em comentário/string não gera falso-positivo.
func TestValidateDbQueries_IgnoresCommentAndString(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/services/user/create/service.go": `package create

// Este service não faz Query( diretamente.
const Doc = "nada de Exec( aqui"
`,
	})
	if err := ValidateDbQueriesInRepositories(root); err != nil {
		t.Fatalf("esperado sem violação para menção textual, got: %v", err)
	}
}
