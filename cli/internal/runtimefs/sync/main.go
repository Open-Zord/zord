// Command sync materializa o runtime subtree do monorepo em
// cli/internal/runtimefs/_runtime, de onde o `zord new` (Fase 1) o embarca via
// //go:embed. É idempotente: limpa o destino e recopia. A localização do
// destino é derivada da raiz do monorepo (go.work), então funciona tanto via
// `go generate` quanto via `go run ./internal/runtimefs/sync`.
//
// Convenções que o embed da Fase 1 exige:
//   - o destino tem prefixo "_" para o go tool não o compilar como pacote do
//     módulo cli (são fontes do runtime, de outro módulo);
//   - go.mod vira go.mod.embed: sem um go.mod o dir não é um módulo aninhado
//     (que o //go:embed se recusaria a atravessar) e o `zord new` reescreve o
//     module path ao gerar o projeto.
package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// runtimeTree lista o subtree que compõe o boilerplate emitido por `zord new`.
// A Fase 1 finaliza este manifesto (ex.: seleção fina de docker/, .env.*); aqui
// ele estabelece o mecanismo. `.gitignore` NÃO entra: o do monorepo tem cruft
// (/zord, _runtime/, etc.) que não deve vazar; um curado é escrito à parte.
var runtimeTree = []string{
	"pkg", "internal", "bootstrap", "cmd/http", "schemas",
	"docker/dev", "docker/prod",
	"compose.yml", ".air.toml",
	".env.example", ".env.http", ".dockerignore",
	"go.mod", "go.sum",
}

// projectGitignore é o .gitignore emitido no projeto gerado — subconjunto
// relevante p/ uma app, sem o cruft de monorepo do .gitignore da raiz.
const projectGitignore = `# .env (creds reais) fica local-only; os .env.<entrypoint> são versionados.
.env
.idea
.vscode/
.DS_Store
storage
tmp/*
temp/
coverage.out
server
`

func main() {
	root, err := findRepoRoot()
	if err != nil {
		log.Fatal(err)
	}
	dest := filepath.Join(root, "cli", "internal", "runtimefs", "_runtime")
	if err := os.RemoveAll(dest); err != nil {
		log.Fatal(err)
	}
	for _, item := range runtimeTree {
		rel := item
		if item == "go.mod" {
			rel = "go.mod.embed"
		}
		if err := copyPath(filepath.Join(root, item), filepath.Join(dest, rel)); err != nil {
			log.Fatalf("copiando %s: %v", item, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dest, ".gitignore"), []byte(projectGitignore), 0o644); err != nil {
		log.Fatalf("escrevendo .gitignore: %v", err)
	}
	log.Printf("runtime materializado em %s (%d entradas + .gitignore curado)", dest, len(runtimeTree))
}

// findRepoRoot sobe a partir do cwd até achar go.work (raiz do monorepo).
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.work não encontrado a partir do cwd")
		}
		dir = parent
	}
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
