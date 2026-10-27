package pkgmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockfileRoundTrip(t *testing.T) {
	root := t.TempDir()

	// Lockfile ausente → vazio, sem erro.
	lf, err := loadLock(root)
	if err != nil {
		t.Fatalf("loadLock vazio: %v", err)
	}
	if len(lf.Packages) != 0 || lf.has("cache") {
		t.Fatal("lockfile ausente deveria vir vazio")
	}

	lf.add(LockedPackage{
		Name:    "cache",
		Package: "cache",
		Files:   map[string]string{"pkg/cache/cache.go": "abc123"},
	})
	if err := lf.save(root); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, lockRelPath)); err != nil {
		t.Fatalf(".zord/packages.lock não foi criado: %v", err)
	}

	got, err := loadLock(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !got.has("cache") {
		t.Fatal("pacote cache deveria estar no lockfile recarregado")
	}
	p := got.get("cache")
	if p == nil || p.Package != "cache" || p.Files["pkg/cache/cache.go"] != "abc123" {
		t.Fatalf("round-trip perdeu dados: %+v", p)
	}

	if !got.removeByName("cache") || got.has("cache") {
		t.Fatal("removeByName deveria remover a entrada")
	}
}

func TestHashFileStable(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "f.txt")
	if err := os.WriteFile(p, []byte("zord"), 0o600); err != nil {
		t.Fatal(err)
	}
	h1, err := hashFile(p)
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}
	h2, _ := hashFile(p)
	if h1 != h2 || h1 == "" {
		t.Fatalf("hash instável/vazio: %q %q", h1, h2)
	}
}
