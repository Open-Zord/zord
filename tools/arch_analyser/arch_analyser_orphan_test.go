package arch_analyser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupBuildableRepo monta um módulo Go mínimo e compilável num TempDir para
// exercitar ValidateNoOrphanPackages, que depende de `go list -json ./...`.
// Diferente de setupFakeRepo (parsing AST puro), aqui os pacotes precisam de
// fato compilar e se importar, porque go list resolve o grafo real do módulo.
func setupBuildableRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/fake\n\ngo 1.22\n"
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

// Caso válido: pacote interno importado por outro pacote interno. Sem órfãos.
func TestValidateNoOrphanPackages_OK(t *testing.T) {
	root := setupBuildableRepo(t, map[string]string{
		// pacote interno usado
		"internal/application/domain/foo/foo.go": `package foo

func Hello() string { return "hi" }
`,
		// pacote interno que importa o anterior (in-degree > 0 para foo)
		"internal/application/services/foo/service.go": `package foo

import dfoo "example.com/fake/internal/application/domain/foo"

func Use() string { return dfoo.Hello() }
`,
		// entrypoint que importa o service (in-degree > 0 para o service)
		"cmd/http/main.go": `package main

import sfoo "example.com/fake/internal/application/services/foo"

func main() { _ = sfoo.Use() }
`,
	})
	if err := ValidateNoOrphanPackages(root); err != nil {
		t.Fatalf("esperava sucesso, deu erro: %v", err)
	}
}

// Violação: pacote interno que ninguém importa.
func TestValidateNoOrphanPackages_DetectaOrfao(t *testing.T) {
	root := setupBuildableRepo(t, map[string]string{
		// pacote interno órfão — ninguém importa
		"internal/application/domain/orphan/orphan.go": `package orphan

func Dead() string { return "dead" }
`,
		// pacote interno ligado, só para garantir que o grafo tem mais de um nó
		"internal/application/domain/used/used.go": `package used

func Alive() string { return "alive" }
`,
		"cmd/http/main.go": `package main

import u "example.com/fake/internal/application/domain/used"

func main() { _ = u.Alive() }
`,
	})
	err := ValidateNoOrphanPackages(root)
	if err == nil {
		t.Fatal("esperava erro de pacote órfão, mas validação passou")
	}
	if !strings.Contains(err.Error(), "pacotes órfãos") {
		t.Fatalf("mensagem inesperada: %v", err)
	}
	if !strings.Contains(err.Error(), "internal/application/domain/orphan") {
		t.Fatalf("erro deveria citar o pacote órfão: %v", err)
	}
	// o pacote ligado não pode aparecer na lista de órfãos
	if strings.Contains(err.Error(), "internal/application/domain/used") {
		t.Fatalf("pacote ligado não deveria ser reportado como órfão: %v", err)
	}
}

// Não-falso-positivo: pacote interno usado SOMENTE por _test.go (TestImports)
// não é órfão. Cobre o caso de mocks/ e helpers de teste.
func TestValidateNoOrphanPackages_UsadoSoPorTesteNaoEhOrfao(t *testing.T) {
	root := setupBuildableRepo(t, map[string]string{
		// helper interno usado apenas por um arquivo _test.go de outro pacote
		"internal/application/domain/helper/helper.go": `package helper

func Fixture() string { return "fixture" }
`,
		// pacote interno "real" ligado ao entrypoint
		"internal/application/domain/foo/foo.go": `package foo

func Hello() string { return "hi" }
`,
		// teste externo (package foo_test) que importa o helper -> XTestImports
		"internal/application/domain/foo/foo_test.go": `package foo_test

import (
	"testing"

	h "example.com/fake/internal/application/domain/helper"
)

func TestHelperUsed(t *testing.T) {
	if h.Fixture() == "" {
		t.Fatal("vazio")
	}
}
`,
		"cmd/http/main.go": `package main

import f "example.com/fake/internal/application/domain/foo"

func main() { _ = f.Hello() }
`,
	})
	if err := ValidateNoOrphanPackages(root); err != nil {
		t.Fatalf("pacote usado só por teste não deveria ser órfão: %v", err)
	}
}

// Exceção: providers padrão do esqueleto zord (pagination, filters) NÃO são
// órfãos mesmo sem importador — vêm com o template (decisão NAVE-127).
func TestValidateNoOrphanPackages_ProvidersPadraoZordExemptos(t *testing.T) {
	root := setupBuildableRepo(t, map[string]string{
		// providers padrão do zord, ambos SEM importador
		"internal/application/providers/pagination/pagination.go": `package pagination

func Paginate() string { return "page" }
`,
		"internal/application/providers/filters/filters.go": `package filters

func Filter() string { return "filter" }
`,
		// pacote real ligado ao entrypoint, pra o grafo ter nó válido
		"internal/application/domain/foo/foo.go": `package foo

func Hello() string { return "hi" }
`,
		"cmd/http/main.go": `package main

import f "example.com/fake/internal/application/domain/foo"

func main() { _ = f.Hello() }
`,
	})
	if err := ValidateNoOrphanPackages(root); err != nil {
		t.Fatalf("providers padrão do zord não deveriam ser órfãos: %v", err)
	}
}
