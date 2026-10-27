package pkgmanager

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/Open-Zord/zord/catalog"
)

// fileState is what Update decides to do with a single installed file.
type fileState int

const (
	stateUnchanged fileState = iota // already matches the catalog
	stateOverwrite                  // unedited since install → re-sync
	stateConflict                   // locally edited → write a .new, keep the file
	stateAdd                        // absent on disk → (re)create
)

// classifyFile decides what to do with one file by comparing the on-disk hash
// (currentHash), the hash recorded at install time (recordedHash) and the
// catalog's current hash (latestHash). onDisk is false when the file is absent.
//
//   - absent               → Add (new in catalog or deleted locally; restore)
//   - current == latest    → Unchanged (includes "edited to match the catalog")
//   - current == recorded  → Overwrite (untouched since install → safe to update)
//   - otherwise            → Conflict (locally edited and the catalog moved)
func classifyFile(onDisk bool, currentHash, recordedHash, latestHash string) fileState {
	if !onDisk {
		return stateAdd
	}
	if currentHash == latestHash {
		return stateUnchanged
	}
	if currentHash == recordedHash {
		return stateOverwrite
	}
	return stateConflict
}

// UpdateResult reports what Update did for one package.
type UpdateResult struct {
	Name        string
	Overwritten []string // re-synced from the catalog
	Conflicts   []string // locally edited; a .new was written next to each
	Added       []string // (re)created
	Unchanged   []string // already up to date
	Removed     []string // recorded but no longer in the catalog (left on disk)
}

// Changed reports whether Update made any change worth mentioning.
func (r UpdateResult) Changed() bool {
	return len(r.Overwritten)+len(r.Conflicts)+len(r.Added)+len(r.Removed) > 0
}

// Update re-syncs an installed package to the embedded catalog, respecting local
// edits: unedited files are overwritten, edited files are left in place and the
// catalog version is written to .zord/updates/<path>.new for manual merge. The
// lockfile hashes are advanced for everything except conflicts (which stay
// flagged until resolved).
func Update(root, name string) (UpdateResult, error) {
	res := UpdateResult{Name: name}
	if root == "" {
		root = "."
	}
	if err := assertZordProject(root); err != nil {
		return res, err
	}
	lock, err := loadLock(root)
	if err != nil {
		return res, err
	}
	locked := lock.get(name)
	if locked == nil {
		return res, fmt.Errorf("pacote %q não está instalado", name)
	}

	manifest, err := catalog.ReadManifest(name)
	if err != nil {
		return res, err
	}
	mod, err := readModule(root)
	if err != nil {
		return res, err
	}
	latest, err := catalogFileContents(name, mod, manifest.Package)
	if err != nil {
		return res, err
	}

	newFiles := make(map[string]string, len(latest))

	for destRel, content := range latest {
		latestHash := hashBytes([]byte(content))
		abs := filepath.Join(root, filepath.FromSlash(destRel))

		onDisk := true
		currentHash := ""
		if h, herr := hashFile(abs); herr == nil {
			currentHash = h
		} else if os.IsNotExist(herr) {
			onDisk = false
		} else {
			return res, herr
		}
		recordedHash := locked.Files[destRel]

		switch classifyFile(onDisk, currentHash, recordedHash, latestHash) {
		case stateUnchanged:
			res.Unchanged = append(res.Unchanged, destRel)
			newFiles[destRel] = latestHash
		case stateOverwrite:
			if err := writeFileAbs(abs, content); err != nil {
				return res, err
			}
			res.Overwritten = append(res.Overwritten, destRel)
			newFiles[destRel] = latestHash
		case stateAdd:
			if err := writeFileAbs(abs, content); err != nil {
				return res, err
			}
			res.Added = append(res.Added, destRel)
			newFiles[destRel] = latestHash
		case stateConflict:
			newPath := filepath.Join(root, ".zord", "updates", filepath.FromSlash(destRel)+".new")
			if err := writeFileAbs(newPath, content); err != nil {
				return res, err
			}
			res.Conflicts = append(res.Conflicts, destRel)
			// Keep the old recorded hash so the file stays flagged until the dev
			// merges the .new and re-runs update.
			newFiles[destRel] = recordedHash
		}
	}

	// Files recorded at install but no longer shipped by the catalog: report and
	// drop from the lock, but leave them on disk (they are the dev's code now).
	for destRel := range locked.Files {
		if _, ok := latest[destRel]; !ok {
			res.Removed = append(res.Removed, destRel)
		}
	}

	locked.Files = newFiles
	if err := lock.save(root); err != nil {
		return res, err
	}

	sort.Strings(res.Overwritten)
	sort.Strings(res.Conflicts)
	sort.Strings(res.Added)
	sort.Strings(res.Unchanged)
	sort.Strings(res.Removed)
	return res, nil
}

// InstalledNames returns the catalog names currently installed at root, sorted.
func InstalledNames(root string) ([]string, error) {
	if root == "" {
		root = "."
	}
	lock, err := loadLock(root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(lock.Packages))
	for _, p := range lock.Packages {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names, nil
}

// catalogFileContents returns the catalog package's files keyed by their
// project-relative slash path (pkg/<pkg>/...), with the catalog module token
// rewritten to the target module. The manifest is skipped.
func catalogFileContents(name, mod, pkg string) (map[string]string, error) {
	srcFS, err := catalog.Files(name)
	if err != nil {
		return nil, err
	}
	repl := []byte(mod + "/pkg/" + pkg)
	tok := []byte(catalogModulePrefix)
	out := map[string]string{}
	err = fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == "." || d.IsDir() || p == catalog.ManifestFileName() {
			return nil
		}
		data, rerr := fs.ReadFile(srcFS, p)
		if rerr != nil {
			return fmt.Errorf("ler %s do catálogo: %w", p, rerr)
		}
		out[path.Join("pkg", pkg, p)] = string(bytes.ReplaceAll(data, tok, repl))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// writeFileAbs writes content to an absolute path, creating parent dirs.
func writeFileAbs(abs, content string) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		return fmt.Errorf("gravar %s: %w", abs, err)
	}
	return nil
}
