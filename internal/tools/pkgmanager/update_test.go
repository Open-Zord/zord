package pkgmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyFile(t *testing.T) {
	cases := []struct {
		name                      string
		onDisk                    bool
		current, recorded, latest string
		want                      fileState
	}{
		{"absent", false, "", "a", "b", stateAdd},
		{"unchanged (matches latest)", true, "b", "a", "b", stateUnchanged},
		{"edited to match latest", true, "b", "a", "b", stateUnchanged},
		{"unedited, catalog moved", true, "a", "a", "b", stateOverwrite},
		{"locally edited, catalog moved", true, "c", "a", "b", stateConflict},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyFile(c.onDisk, c.current, c.recorded, c.latest)
			if got != c.want {
				t.Fatalf("classifyFile(%v,%q,%q,%q) = %d, want %d", c.onDisk, c.current, c.recorded, c.latest, got, c.want)
			}
		})
	}
}

// newProjectWithCache monta um projeto mínimo e instala o cache nele.
func newProjectWithCache(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/t\n\ngo 1.26\n")
	mustWrite(t, filepath.Join(root, "bootstrap", "http", "pkg.go"),
		"package http\n\nfunc registerPkg(reg *Registry, conf *Config) {\n}\n")
	if _, err := Add(root, "cache"); err != nil {
		t.Fatalf("Add cache: %v", err)
	}
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestUpdate_Unchanged(t *testing.T) {
	root := newProjectWithCache(t)
	res, err := Update(root, "cache")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.Changed() {
		t.Fatalf("esperava nada a mudar, veio: %+v", res)
	}
}

func TestUpdate_Conflict(t *testing.T) {
	root := newProjectWithCache(t)
	cacheGo := filepath.Join(root, "pkg", "cache", "cache.go")

	edited := "package cache\n\n// edição local do dev\nfunc Extra() {}\n"
	mustWrite(t, cacheGo, edited)

	res, err := Update(root, "cache")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "pkg/cache/cache.go" {
		t.Fatalf("esperava conflito em pkg/cache/cache.go, veio: %+v", res)
	}
	// O arquivo editado NÃO deve ter sido tocado.
	got, _ := os.ReadFile(cacheGo)
	if string(got) != edited {
		t.Fatal("arquivo editado foi sobrescrito (não deveria)")
	}
	// A versão do catálogo deve ter ido para .new.
	newPath := filepath.Join(root, ".zord", "updates", "pkg", "cache", "cache.go.new")
	nb, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("esperava .new em %s: %v", newPath, err)
	}
	if !strings.Contains(string(nb), "RegistryKey") {
		t.Fatal(".new não tem a versão do catálogo")
	}
}

func TestUpdate_Overwrite(t *testing.T) {
	root := newProjectWithCache(t)
	cacheGo := filepath.Join(root, "pkg", "cache", "cache.go")

	// Simula "catálogo avançou e o dev não editou": o conteúdo em disco == hash
	// gravado (unedited), mas difere do catálogo. Fazemos isso gravando um
	// conteúdo antigo X e marcando o lockfile como se X fosse o instalado.
	old := "package cache\n\n// versão antiga\n"
	mustWrite(t, cacheGo, old)
	lock, err := loadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.get("cache").Files["pkg/cache/cache.go"] = hashBytes([]byte(old))
	if err := lock.save(root); err != nil {
		t.Fatal(err)
	}

	res, err := Update(root, "cache")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("não deveria haver conflito (unedited): %+v", res)
	}
	found := false
	for _, f := range res.Overwritten {
		if f == "pkg/cache/cache.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperava overwrite de pkg/cache/cache.go, veio: %+v", res)
	}
	got, _ := os.ReadFile(cacheGo)
	if !strings.Contains(string(got), "RegistryKey") {
		t.Fatal("arquivo não foi re-sincronizado com o catálogo")
	}
}
