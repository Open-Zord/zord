package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newFakeRepo cria um diretório temporário com go.mod, simulando um repo Go
// válido. Devolve o path absoluto.
func newFakeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fake\n"), 0o600); err != nil {
		t.Fatalf("escrever go.mod: %v", err)
	}
	return dir
}

func TestEffectiveRepo_OverrideVazioDevolveDefault(t *testing.T) {
	def := newFakeRepo(t)

	got, err := effectiveRepo("", def)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != def {
		t.Fatalf("esperado %q, obtido %q", def, got)
	}
}

func TestEffectiveRepo_OverrideAbsolutoValido(t *testing.T) {
	def := newFakeRepo(t)
	override := newFakeRepo(t)

	got, err := effectiveRepo(override, def)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != override {
		t.Fatalf("esperado override %q, obtido %q", override, got)
	}
}

func TestEffectiveRepo_OverrideRelativoViraAbsoluto(t *testing.T) {
	def := newFakeRepo(t)
	override := newFakeRepo(t)

	// cd para o pai do override e usa o nome relativo
	parent := filepath.Dir(override)
	name := filepath.Base(override)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(parent); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := effectiveRepo(name, def)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("path devolvido não é absoluto: %q", got)
	}
	// Resolve symlinks pra comparar (em macOS /tmp é symlink, mas TempDir já
	// resolve; ainda assim mantemos defensivo)
	wantAbs, _ := filepath.Abs(filepath.Join(parent, name))
	if got != wantAbs {
		t.Fatalf("esperado %q, obtido %q", wantAbs, got)
	}
}

func TestEffectiveRepo_OverrideInexistenteFalha(t *testing.T) {
	def := newFakeRepo(t)
	missing := filepath.Join(t.TempDir(), "nao-existe")

	_, err := effectiveRepo(missing, def)
	if err == nil {
		t.Fatal("esperado erro de path inexistente, obtido nil")
	}
	if !strings.Contains(err.Error(), "acessar repo") {
		t.Fatalf("mensagem inesperada: %v", err)
	}
}

func TestEffectiveRepo_OverrideArquivoFalha(t *testing.T) {
	def := newFakeRepo(t)
	file := filepath.Join(t.TempDir(), "arquivo.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("escrever arquivo: %v", err)
	}

	_, err := effectiveRepo(file, def)
	if err == nil {
		t.Fatal("esperado erro, obtido nil")
	}
	if !strings.Contains(err.Error(), "não é diretório") {
		t.Fatalf("mensagem inesperada: %v", err)
	}
}

func TestEffectiveRepo_OverrideSemGoModFalha(t *testing.T) {
	def := newFakeRepo(t)
	dir := t.TempDir() // sem go.mod

	_, err := effectiveRepo(dir, def)
	if err == nil {
		t.Fatal("esperado erro, obtido nil")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("mensagem inesperada: %v", err)
	}
}
