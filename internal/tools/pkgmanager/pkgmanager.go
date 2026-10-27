// Package pkgmanager implements `zord package`: it installs, lists and removes
// Zord batteries from the embedded catalog into a project. Installing copies a
// package's source into pkg/<name>/, wires its constructor into the DI registry
// (via the scaffold AST machinery) and records the install in .zord/packages.lock
// — "code as asset": the package becomes the project's own code.
package pkgmanager

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-Zord/zord/catalog"
	"github.com/Open-Zord/zord/internal/tools/scaffold"
)

// catalogModulePrefix is the import prefix under which catalog packages compile
// inside the monorepo. On install it is rewritten to <projectModule>/pkg/<pkg>
// so a battery that imports its own subpackages resolves in the target project.
// Self-contained batteries (like cache) contain no occurrence — a no-op.
const catalogModulePrefix = "github.com/Open-Zord/zord/catalog"

// AddResult reports what `Add` did.
type AddResult struct {
	Package   string   // destination package name (pkg/<Package>/)
	Files     []string // project-relative paths written
	Bootstrap string   // relative path of the edited bootstrap file
}

// Add installs the catalog package `name` into the project at root: copies its
// files into pkg/<package>/, wires the constructor into registerPkg and records
// the install in the lockfile. It refuses if the package is already installed
// or the destination directory already exists.
func Add(root, name string) (AddResult, error) {
	var res AddResult
	if root == "" {
		root = "."
	}
	if err := assertZordProject(root); err != nil {
		return res, err
	}

	manifest, err := catalog.ReadManifest(name)
	if err != nil {
		return res, err
	}

	lock, err := loadLock(root)
	if err != nil {
		return res, err
	}
	if lock.has(name) {
		return res, fmt.Errorf("pacote %q já instalado (veja .zord/packages.lock)", name)
	}

	destRel := filepath.Join("pkg", manifest.Package)
	destAbs := filepath.Join(root, destRel)
	if _, err := os.Stat(destAbs); err == nil {
		return res, fmt.Errorf("%s já existe — remova-o antes de instalar %q", destRel, name)
	}

	mod, err := readModule(root)
	if err != nil {
		return res, err
	}
	srcFS, err := catalog.Files(name)
	if err != nil {
		return res, err
	}
	repl := mod + "/pkg/" + manifest.Package
	files, err := copyPackageFiles(srcFS, root, destRel, catalogModulePrefix, repl)
	if err != nil {
		return res, err
	}

	// Wire into the DI. On failure, roll back the copied files so the install is
	// all-or-nothing.
	editedRel, err := scaffold.RegisterPkg(scaffold.RegisterPkgOptions{
		Root:        root,
		Package:     manifest.Package,
		Constructor: manifest.Constructor,
	})
	if err != nil {
		_ = os.RemoveAll(destAbs)
		return res, fmt.Errorf("conectar %q no registry: %w", name, err)
	}

	locked := LockedPackage{Name: name, Package: manifest.Package, Files: map[string]string{}}
	for _, rel := range files {
		sum, herr := hashFile(filepath.Join(root, rel))
		if herr != nil {
			return res, fmt.Errorf("hash de %s: %w", rel, herr)
		}
		locked.Files[rel] = sum
	}
	lock.add(locked)
	if err := lock.save(root); err != nil {
		return res, err
	}

	return AddResult{Package: manifest.Package, Files: files, Bootstrap: editedRel}, nil
}

// Remove uninstalls the catalog package `name`: unwires it from registerPkg,
// deletes pkg/<package>/ and drops the lockfile entry. It refuses if the package
// is not installed.
func Remove(root, name string) error {
	if root == "" {
		root = "."
	}
	if err := assertZordProject(root); err != nil {
		return err
	}

	lock, err := loadLock(root)
	if err != nil {
		return err
	}
	locked := lock.get(name)
	if locked == nil {
		return fmt.Errorf("pacote %q não está instalado", name)
	}

	if _, err := scaffold.UnregisterPkg(scaffold.RegisterPkgOptions{
		Root:    root,
		Package: locked.Package,
	}); err != nil {
		return fmt.Errorf("desconectar %q do registry: %w", name, err)
	}

	if err := os.RemoveAll(filepath.Join(root, "pkg", locked.Package)); err != nil {
		return fmt.Errorf("remover pkg/%s: %w", locked.Package, err)
	}

	lock.removeByName(name)
	return lock.save(root)
}

// ListEntry is one catalog package with its installed status.
type ListEntry struct {
	Name        string
	Description string
	Installed   bool
}

// List returns the catalog packages, marking which are installed in the project
// at root. It works even outside a project (nothing marked installed).
func List(root string) ([]ListEntry, error) {
	if root == "" {
		root = "."
	}
	names, err := catalog.Names()
	if err != nil {
		return nil, err
	}
	lock, err := loadLock(root)
	if err != nil {
		return nil, err
	}
	entries := make([]ListEntry, 0, len(names))
	for _, n := range names {
		m, err := catalog.ReadManifest(n)
		if err != nil {
			return nil, err
		}
		entries = append(entries, ListEntry{Name: m.Name, Description: m.Description, Installed: lock.has(n)})
	}
	return entries, nil
}

// assertZordProject checks root looks like a Zord project: a go.mod and the
// bootstrap wiring file the installer edits.
func assertZordProject(root string) error {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("%s não parece um projeto Go (go.mod ausente)", root)
	}
	if _, err := os.Stat(filepath.Join(root, "bootstrap", "http", "pkg.go")); err != nil {
		return fmt.Errorf("%s não parece um projeto Zord (bootstrap/http/pkg.go ausente)", root)
	}
	return nil
}

// copyPackageFiles writes srcFS (a catalog package's files) into root/destRel,
// skipping the manifest, rewriting the token → repl in each file. Returns the
// project-relative paths written.
func copyPackageFiles(srcFS fs.FS, root, destRel, token, repl string) ([]string, error) {
	var written []string
	tok, rep := []byte(token), []byte(repl)
	err := fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." || d.IsDir() || path == catalog.ManifestFileName() {
			return nil
		}
		data, err := fs.ReadFile(srcFS, path)
		if err != nil {
			return fmt.Errorf("ler %s do catálogo: %w", path, err)
		}
		data = bytes.ReplaceAll(data, tok, rep)
		rel := filepath.Join(destRel, filepath.FromSlash(path))
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(abs, data, 0o600); err != nil {
			return fmt.Errorf("gravar %s: %w", rel, err)
		}
		written = append(written, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}

// readModule returns the module path declared in root/go.mod.
func readModule(root string) (string, error) {
	f, err := os.Open(filepath.Join(root, "go.mod")) //nolint:gosec // path derives from the repo root
	if err != nil {
		return "", fmt.Errorf("abrir go.mod: %w", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("diretiva module não encontrada em go.mod")
}
