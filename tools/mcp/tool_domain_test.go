package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-Zord/zord/tools/scaffold"
)

// TestDomainCreate_RepoOverrideDirecionaParaTarget é um smoke test que valida
// que o helper effectiveRepo + scaffold.DomainCreate combinados redirecionam a
// geração para o path passado por chamada, mesmo quando o default aponta para
// outro repo. Reproduz o caminho real do handler de scaffold_domain_create sem
// subir o servidor MCP.
func TestDomainCreate_RepoOverrideDirecionaParaTarget(t *testing.T) {
	def := newFakeRepo(t)      // simula o --repo do startup
	override := newFakeRepo(t) // simula o `repo` passado por chamada

	target, err := effectiveRepo(override, def)
	if err != nil {
		t.Fatalf("effectiveRepo: %v", err)
	}
	if target != override {
		t.Fatalf("esperado target %q, obtido %q", override, target)
	}

	rel, err := scaffold.DomainCreate("OrgMembership", scaffold.DomainCreateOptions{Root: target})
	if err != nil {
		t.Fatalf("DomainCreate: %v", err)
	}

	wantFile := filepath.Join(override, rel)
	if _, err := os.Stat(wantFile); err != nil {
		t.Fatalf("arquivo esperado no override %q ausente: %v", wantFile, err)
	}

	// O default NÃO deve ter sido tocado.
	defFile := filepath.Join(def, rel)
	if _, err := os.Stat(defFile); !os.IsNotExist(err) {
		t.Fatalf("default %q não deveria ter o arquivo, mas tem (ou erro inesperado: %v)", defFile, err)
	}
}

// TestDomainCreate_RepoVazioCaiNoDefault valida que omitir Repo na chamada faz
// a geração ir para o default do startup.
func TestDomainCreate_RepoVazioCaiNoDefault(t *testing.T) {
	def := newFakeRepo(t)

	target, err := effectiveRepo("", def)
	if err != nil {
		t.Fatalf("effectiveRepo: %v", err)
	}
	if target != def {
		t.Fatalf("esperado default %q, obtido %q", def, target)
	}

	rel, err := scaffold.DomainCreate("Widget", scaffold.DomainCreateOptions{Root: target})
	if err != nil {
		t.Fatalf("DomainCreate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(def, rel)); err != nil {
		t.Fatalf("arquivo no default ausente: %v", err)
	}
}
