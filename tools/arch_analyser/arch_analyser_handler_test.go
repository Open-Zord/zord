package arch_analyser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupFakeRepo monta uma raiz mínima com go.mod e a árvore cmd/http/handlers
// pra testar ValidateNoHandlerCrossImports isoladamente.
func setupFakeRepo(t *testing.T, files map[string]string) string {
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

func TestValidateNoHandlerCrossImports_OK(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"cmd/http/handlers/foo/create/handler.go": `package create

type CreateHandler struct{}
`,
		"cmd/http/handlers/foo/list/handler.go": `package list

type ListHandler struct{}
`,
		// subpacote auxiliar (não tem *Handler) importado por um handler.
		// ValidateNoHandlerCrossImports continua passando aqui — ele só barra
		// import entre pacotes que declaram *Handler (decisão 5: defesa em
		// profundidade). O furo estrutural (subpacote auxiliar existir) é
		// fechado por ValidateScaffoldLayout — ver teste de inversão abaixo.
		"cmd/http/handlers/foo/shared/util.go": `package shared

func Helper() {}
`,
		"cmd/http/handlers/foo/get/handler.go": `package get

import _ "example.com/fake/cmd/http/handlers/foo/shared"

type GetHandler struct{}
`,
	})
	if err := ValidateNoHandlerCrossImports(root); err != nil {
		t.Fatalf("esperava sucesso, deu erro: %v", err)
	}
}

// TestValidateScaffoldLayout_RejeitaSubpacoteAuxiliar inverte a premissa
// histórica da NAVE-123: o subpacote auxiliar shared/ que passava livre no
// enforce por import agora é REPROVADO pelo enforce de layout (NAVE-126). É a
// mesma classe de qualquer subpacote auxiliar dentro de handlers/<domain>/.
func TestValidateScaffoldLayout_RejeitaSubpacoteAuxiliar(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/services/foo/create/service.go": "package create\n",
		"internal/application/services/foo/get/service.go":    "package get\n",
		"cmd/http/handlers/foo/create/handler.go": `package create

type CreateHandler struct{}
`,
		"cmd/http/handlers/foo/get/handler.go": `package get

type GetHandler struct{}
`,
		// subpacote auxiliar — agora violação de layout
		"cmd/http/handlers/foo/shared/util.go": `package shared

func Helper() {}
`,
	})
	err := ValidateScaffoldLayout(root)
	if err == nil {
		t.Fatal("esperava reprovação do subpacote auxiliar em handlers/, mas passou")
	}
	if !strings.Contains(err.Error(), "cmd/http/handlers/foo/shared") {
		t.Fatalf("erro deveria citar o subpacote auxiliar: %v", err)
	}
}

func TestValidateNoHandlerCrossImports_DetectaImportCruzado(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"cmd/http/handlers/foo/create/handler.go": `package create

type CreateHandler struct{}
`,
		"cmd/http/handlers/foo/list/handler.go": `package list

import _ "example.com/fake/cmd/http/handlers/foo/create"

type ListHandler struct{}
`,
	})
	err := ValidateNoHandlerCrossImports(root)
	if err == nil {
		t.Fatal("esperava erro de import cruzado, mas validação passou")
	}
	if !strings.Contains(err.Error(), "handler importa outro handler") {
		t.Fatalf("mensagem inesperada: %v", err)
	}
	if !strings.Contains(err.Error(), "cmd/http/handlers/foo/create") {
		t.Fatalf("erro deveria citar o handler importado: %v", err)
	}
}

func TestValidateNoHandlerCrossImports_SemHandlersDir(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"main.go": "package main\n",
	})
	if err := ValidateNoHandlerCrossImports(root); err != nil {
		t.Fatalf("repo sem cmd/http/handlers/ não deveria falhar: %v", err)
	}
}
