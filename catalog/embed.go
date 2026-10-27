// Package catalog embeds the Zord package catalog — the curated "batteries"
// that `zord package add` installs into a project. Each package is a directory
// under catalog/ with a zpack.toml manifest plus the Go source that becomes the
// project's pkg/<package>/. The sources also compile as part of this module, so
// they are dogfooded (built and tested) in the repo.
//
// Adding a package = create catalog/<name>/ and extend the embed directive with
// `all:<name>` (mirrors the providers list in internal/commands/root.go).
package catalog

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// manifestFile is the per-package manifest name within catalog/<name>/.
const manifestFile = "zpack.toml"

//go:embed all:cache
var embedded embed.FS

// Manifest describes a catalog package. It is the parsed zpack.toml.
type Manifest struct {
	// Name is the package name as used on the CLI (`zord package add <name>`).
	Name string `toml:"name"`
	// Description is a one-line summary shown by `zord package list`.
	Description string `toml:"description"`
	// Package is the destination directory/import name: files land in
	// pkg/<Package>/ and are wired as <Package>.<Constructor>().
	Package string `toml:"package"`
	// Constructor is the no-arg constructor registered in the registry.
	Constructor string `toml:"constructor"`
}

// Names returns the catalog package names (directory names), sorted.
func Names() ([]string, error) {
	entries, err := fs.ReadDir(embedded, ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// ReadManifest parses the manifest of the named catalog package.
func ReadManifest(name string) (Manifest, error) {
	var m Manifest
	data, err := fs.ReadFile(embedded, name+"/"+manifestFile)
	if err != nil {
		return m, fmt.Errorf("pacote %q não encontrado no catálogo: %w", name, err)
	}
	if err := toml.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("manifesto de %q inválido: %w", name, err)
	}
	return m, nil
}

// Files returns an fs.FS rooted at the named package's directory — its contents
// (including the manifest) are what `zord package add` copies into the project,
// minus the manifest itself (see ManifestFileName).
func Files(name string) (fs.FS, error) {
	sub, err := fs.Sub(embedded, name)
	if err != nil {
		return nil, fmt.Errorf("pacote %q não encontrado no catálogo: %w", name, err)
	}
	return sub, nil
}

// ManifestFileName is the manifest filename callers must skip when copying a
// package's Files() into a project.
func ManifestFileName() string { return manifestFile }
