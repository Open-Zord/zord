// Package projectgen materializa um projeto novo a partir do runtime embarcado:
// percorre o FS, reescreve o module path e grava os arquivos no destino.
package projectgen

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// sourceModule é o module path do runtime embarcado (o do monorepo). A reescrita
// é substituição de string deste token — robusta e sem sintaxe de template:
// github.com/Open-Zord/zord/pkg/x -> <module>/pkg/x, e `module ...` no go.mod.
const sourceModule = "github.com/Open-Zord/zord"

// Generate escreve o conteúdo de src (o FS do runtime) em destDir, reescrevendo
// sourceModule -> module em cada arquivo e gravando go.mod.embed como go.mod.
// destDir deve estar vazio/inexistente (validado pelo chamador).
func Generate(src fs.FS, destDir, module string) error {
	token := []byte(sourceModule)
	repl := []byte(module)

	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}

		dst := filepath.Join(destDir, path)
		if path == "go.mod.embed" {
			dst = filepath.Join(destDir, "go.mod")
		}

		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}

		data, err := fs.ReadFile(src, path)
		if err != nil {
			return fmt.Errorf("lendo %s: %w", path, err)
		}
		data = bytes.ReplaceAll(data, token, repl)

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("gravando %s: %w", dst, err)
		}
		return nil
	})
}
