package upgrader

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// WalkGoFiles chama fn para cada arquivo .go sob root, pulando diretórios que um
// codemod nunca deve tocar (.zord, .git, vendor, testdata, node_modules). É o
// percurso-base de um codemod que varre o projeto.
func WalkGoFiles(root string, fn func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".zord", ".git", "vendor", "testdata", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			return fn(path)
		}
		return nil
	})
}

// RewriteGoFile parseia o arquivo Go em path (com comentários), chama transform
// no AST e, se transform reportar changed, reescreve o arquivo formatado
// (go/format). Não toca o arquivo se nada mudou — upgrades idempotentes não
// churnam o diff. É o helper central para um codemod que edita um arquivo.
func RewriteGoFile(path string, transform func(file *ast.File) (changed bool, err error)) error {
	fset := token.NewFileSet()
	src, err := os.ReadFile(path) //nolint:gosec // path vem do walk sob a raiz do projeto
	if err != nil {
		return fmt.Errorf("ler %s: %w", path, err)
	}
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	changed, err := transform(file)
	if err != nil {
		return fmt.Errorf("transformar %s: %w", path, err)
	}
	if !changed {
		return nil
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt %s: %w", path, err)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil { //nolint:gosec // boilerplate do projeto do dev, 0644 proposital
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	return nil
}
