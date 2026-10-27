package pkgmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// lockRelPath is the project-relative path of the package lockfile. It is
// committed with the project and records which catalog packages are installed
// and the sha256 of each file as installed (the baseline for drift detection
// on a future `zord package update`).
const lockRelPath = ".zord/packages.lock"

// Lockfile is the parsed .zord/packages.lock.
type Lockfile struct {
	Packages []LockedPackage `toml:"package"`
}

// LockedPackage records one installed package.
type LockedPackage struct {
	// Name is the catalog name (`zord package add <Name>`).
	Name string `toml:"name"`
	// Package is the destination directory under pkg/ (import name).
	Package string `toml:"package"`
	// Files maps each installed project-relative path to its sha256 at install
	// time, sorted by path when written for a stable diff.
	Files map[string]string `toml:"files"`
}

// loadLock reads the lockfile at root. A missing lockfile is not an error — it
// returns an empty Lockfile.
func loadLock(root string) (*Lockfile, error) {
	path := filepath.Join(root, lockRelPath)
	data, err := os.ReadFile(path) //nolint:gosec // path derives from the repo root
	if os.IsNotExist(err) {
		return &Lockfile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", lockRelPath, err)
	}
	var lf Lockfile
	if err := toml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("%s inválido: %w", lockRelPath, err)
	}
	return &lf, nil
}

// save writes the lockfile under root, creating .zord/ if needed. Packages are
// sorted by name for a stable, diff-friendly file.
func (lf *Lockfile) save(root string) error {
	sort.Slice(lf.Packages, func(i, j int) bool { return lf.Packages[i].Name < lf.Packages[j].Name })
	path := filepath.Join(root, lockRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("criar %s: %w", filepath.Dir(lockRelPath), err)
	}
	data, err := toml.Marshal(lf)
	if err != nil {
		return fmt.Errorf("serializar lockfile: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("gravar %s: %w", lockRelPath, err)
	}
	return nil
}

// has reports whether a package with the given catalog name is installed.
func (lf *Lockfile) has(name string) bool {
	for i := range lf.Packages {
		if lf.Packages[i].Name == name {
			return true
		}
	}
	return false
}

// get returns the locked package with the given name, or nil.
func (lf *Lockfile) get(name string) *LockedPackage {
	for i := range lf.Packages {
		if lf.Packages[i].Name == name {
			return &lf.Packages[i]
		}
	}
	return nil
}

// add appends a locked package (caller ensures it is not already present).
func (lf *Lockfile) add(p LockedPackage) {
	lf.Packages = append(lf.Packages, p)
}

// removeByName drops the package with the given name; returns false if absent.
func (lf *Lockfile) removeByName(name string) bool {
	for i := range lf.Packages {
		if lf.Packages[i].Name == name {
			lf.Packages = append(lf.Packages[:i], lf.Packages[i+1:]...)
			return true
		}
	}
	return false
}

// hashBytes returns the hex-encoded sha256 of data.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hashFile returns the hex-encoded sha256 of the file at path.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path derives from the repo root
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}
