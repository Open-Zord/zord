package arch_analyser

import (
	"strings"
	"testing"
)

// Regressão do match permissivo por strings.Contains: um pacote-irmão com
// prefixo comum (domain_evil) NÃO deve ser aceito como se fosse o pacote da
// camada permitida (domain).
func TestValidateLayerDependencies_RejectsSiblingPrefix(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/repositories/user/user.go": `package userrepo

import _ "example.com/fake/internal/application/domain_evil/x"
`,
	})
	err := ValidateLayerDependencies(root)
	if err == nil || !strings.Contains(err.Error(), "domain_evil") {
		t.Fatalf("esperado violação para domain_evil, got: %v", err)
	}
}

// O pacote de domínio legítimo (segmento inteiro) continua permitido em
// repositories/.
func TestValidateLayerDependencies_AllowsRealDomain(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/repositories/user/user.go": `package userrepo

import _ "example.com/fake/internal/application/domain/user"
`,
	})
	if err := ValidateLayerDependencies(root); err != nil {
		t.Fatalf("esperado sem violação para domain legítimo, got: %v", err)
	}
}

// Testa o casamento por borda de segmento diretamente.
func TestImportMatchesLayer(t *testing.T) {
	const allowed = "internal/application/domain"
	match := []string{
		"internal/application/domain",
		"example.com/m/internal/application/domain",
		"example.com/m/internal/application/domain/user",
		"internal/application/domain/user",
	}
	for _, imp := range match {
		if !importMatchesLayer(imp, allowed) {
			t.Errorf("importMatchesLayer(%q, %q) = false; want true", imp, allowed)
		}
	}
	noMatch := []string{
		"internal/application/domain_evil",
		"example.com/m/internal/application/domain_evil/x",
		"internal/application/domainextra",
		"example.com/m/notinternal/application/domain", // só casa com borda
	}
	for _, imp := range noMatch {
		if importMatchesLayer(imp, allowed) {
			t.Errorf("importMatchesLayer(%q, %q) = true; want false", imp, allowed)
		}
	}
}
