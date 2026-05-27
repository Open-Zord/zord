// Package scaffold (área domain) constrói e edita artefatos de domínio Go via AST.
package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
)

// BasePath é o diretório, relativo à raiz do repositório, onde vivem os
// pacotes de domínio do backend.
const BasePath = "internal/application/domain"

// DomainCreateOptions parametriza DomainCreate.
type DomainCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
}

// DomainCreate gera o arquivo de domínio para typeName, contendo apenas o pacote
// e uma struct vazia exportada. Retorna o caminho relativo à raiz.
// Falha se o arquivo já existe.
func DomainCreate(typeName string, opts DomainCreateOptions) (string, error) {
	if !IsValidExportedIdent(typeName) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", typeName)
	}
	pkg := ToSnake(typeName)
	root := opts.Root
	if root == "" {
		root = "."
	}

	relDir := filepath.Join(BasePath, pkg)
	relFile := filepath.Join(relDir, pkg+".go")
	absDir := filepath.Join(root, relDir)
	absFile := filepath.Join(root, relFile)

	if _, err := os.Stat(absFile); err == nil {
		return "", fmt.Errorf("domínio já existe: %s", relFile)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", absFile, err)
	}

	src, err := buildSource(pkg, typeName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", absDir, err)
	}
	if err := os.WriteFile(absFile, src, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", absFile, err)
	}
	return relFile, nil
}

func buildSource(pkg, typeName string) ([]byte, error) {
	emptyStruct := &ast.StructType{Fields: FieldList()}
	f := &ast.File{
		Name:  Ident(pkg),
		Decls: []ast.Decl{TypeDecl(typeName, emptyStruct)},
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), f); err != nil {
		return nil, fmt.Errorf("formatar AST: %w", err)
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
