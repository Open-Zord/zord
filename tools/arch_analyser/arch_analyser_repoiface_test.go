package arch_analyser

import (
	"strings"
	"testing"
)

// Interface de repositório sem struct correspondente em repositories/ deve
// acusar violação.
func TestValidateRepositoryInterfacesImplemented_Missing(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/domain/user/user.go": `package user

type UserRepository interface {
	Save() error
}
`,
		"internal/repositories/user/user.go": `package userrepo

type userRepo struct {
	id string
}
`,
	})
	err := ValidateRepositoryInterfacesImplemented(root)
	if err == nil || !strings.Contains(err.Error(), "UserRepository") {
		t.Fatalf("esperado violação mencionando UserRepository, got: %v", err)
	}
}

// Com um campo nomeado igual à interface (heurística atual), passa.
func TestValidateRepositoryInterfacesImplemented_Found(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/domain/user/user.go": `package user

type UserRepository interface {
	Save() error
}
`,
		"internal/repositories/user/user.go": `package userrepo

type userRepo struct {
	UserRepository string
}
`,
	})
	if err := ValidateRepositoryInterfacesImplemented(root); err != nil {
		t.Fatalf("esperado sem violação, got: %v", err)
	}
}

// Regressão do cast inseguro: tipos não-struct em repositories/ (alias,
// interface, etc.) não podem derrubar o validador por panic.
func TestValidateRepositoryInterfacesImplemented_NoPanicOnNonStruct(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/domain/user/user.go": `package user

type UserRepository interface {
	Save() error
}
`,
		"internal/repositories/user/user.go": `package userrepo

type alias = int

type marker interface{ m() }

type count int
`,
	})
	// Sem struct com o campo -> violação esperada, mas sobretudo: sem panic.
	if err := ValidateRepositoryInterfacesImplemented(root); err == nil {
		t.Fatal("esperado violação (nenhuma struct implementa), got nil")
	}
}
