package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// seedPkg grava pkg/<name>/<name>.go expondo RegistryKey + construtor.
func seedPkg(t *testing.T, root, name, ctor string) {
	t.Helper()
	absDir := filepath.Join(root, "pkg", name)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fmt.Sprintf("package %s\n\nconst RegistryKey = %q\n\ntype T struct{}\n\nfunc %s() *T { return &T{} }\n", name, name, ctor)
	if err := os.WriteFile(filepath.Join(absDir, name+".go"), []byte(src), 0o600); err != nil {
		t.Fatalf("seed pkg: %v", err)
	}
}

// seedBootstrapPkg grava um bootstrap/http/pkg.go mínimo com registerPkg vazio.
func seedBootstrapPkg(t *testing.T, root string) {
	t.Helper()
	absFile := filepath.Join(root, "bootstrap", "http", "pkg.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package http

import (
	"zord/pkg/config"
	"zord/pkg/registry"
)

func registerPkg(reg *registry.Registry, conf *config.Config) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("seed bootstrap/http/pkg.go: %v", err)
	}
}

func TestRegisterPkg_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedPkg(t, root, "cache", "NewCache")
	seedBootstrapPkg(t, root)

	rel, err := RegisterPkg(RegisterPkgOptions{Root: root, Package: "cache", Constructor: "NewCache"})
	if err != nil {
		t.Fatalf("RegisterPkg: %v", err)
	}
	if want := filepath.Join("bootstrap", "http", "pkg.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`"zord/pkg/cache"`,
		"reg.Provide(cache.RegistryKey, cache.NewCache())",
	)
	mustParse(t, got)
}

func TestRegisterPkg_Idempotente(t *testing.T) {
	root := t.TempDir()
	seedPkg(t, root, "cache", "NewCache")
	seedBootstrapPkg(t, root)
	if _, err := RegisterPkg(RegisterPkgOptions{Root: root, Package: "cache", Constructor: "NewCache"}); err != nil {
		t.Fatalf("1ª RegisterPkg: %v", err)
	}
	if _, err := RegisterPkg(RegisterPkgOptions{Root: root, Package: "cache", Constructor: "NewCache"}); err == nil {
		t.Fatal("2ª RegisterPkg deveria falhar (já registrado)")
	}
}

func TestRegisterPkg_PacoteAusente(t *testing.T) {
	root := t.TempDir()
	seedBootstrapPkg(t, root) // sem pkg/cache/
	if _, err := RegisterPkg(RegisterPkgOptions{Root: root, Package: "cache", Constructor: "NewCache"}); err == nil {
		t.Fatal("esperava erro por pkg/cache ausente")
	}
}

func TestRegisterPkg_NomeInvalido(t *testing.T) {
	root := t.TempDir()
	seedBootstrapPkg(t, root)
	if _, err := RegisterPkg(RegisterPkgOptions{Root: root, Package: "123bad", Constructor: "NewCache"}); err == nil {
		t.Fatal("esperava erro por nome de pacote inválido")
	}
}

func TestUnregisterPkg_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedPkg(t, root, "cache", "NewCache")
	seedBootstrapPkg(t, root)
	if _, err := RegisterPkg(RegisterPkgOptions{Root: root, Package: "cache", Constructor: "NewCache"}); err != nil {
		t.Fatalf("RegisterPkg: %v", err)
	}

	if _, err := UnregisterPkg(RegisterPkgOptions{Root: root, Package: "cache"}); err != nil {
		t.Fatalf("UnregisterPkg: %v", err)
	}
	got := readFile(t, filepath.Join(root, "bootstrap", "http", "pkg.go"))
	mustNotContain(t, got, "zord/pkg/cache", "cache.RegistryKey")
	mustParse(t, got)
}

func TestUnregisterPkg_NaoRegistrado(t *testing.T) {
	root := t.TempDir()
	seedBootstrapPkg(t, root)
	if _, err := UnregisterPkg(RegisterPkgOptions{Root: root, Package: "cache"}); err == nil {
		t.Fatal("esperava erro: nada a desregistrar")
	}
}
