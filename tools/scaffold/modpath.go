package scaffold

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultModulePath é o prefixo usado quando a raiz não tem go.mod. Repos reais
// sempre têm go.mod, então este fallback só vale em fixtures de teste, onde o
// objetivo é apenas ter um prefixo estável e determinístico.
const defaultModulePath = "zord"

// modulePath lê o `module <path>` do go.mod na raiz informada e retorna o path
// do module. root vazio resolve para ".". É a fonte de verdade do prefixo de
// import usado por todo o scaffold: o código gerado num repo qualquer importa
// `<module>/internal/...` em vez de um valor hardcoded. Quando o go.mod não
// existe, cai no defaultModulePath (cenário de teste).
func modulePath(root string) (string, error) {
	if root == "" {
		root = "."
	}
	goMod := filepath.Join(root, "go.mod")
	//nolint:gosec // G304: path derives from the repo root, not raw user input
	f, err := os.Open(goMod)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultModulePath, nil
		}
		return "", fmt.Errorf("abrir go.mod: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if path == "" {
				return "", fmt.Errorf("em %s: diretiva module sem valor", goMod)
			}
			return path, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("ler %s: %w", goMod, err)
	}
	return "", fmt.Errorf("em %s: diretiva module não encontrada", goMod)
}

// importPaths agrupa os prefixos de import que o scaffold usa, todos derivados
// do module path do repo alvo. Substitui os antigos const hardcoded.
type importPaths struct {
	module string
}

func newImportPaths(root string) (importPaths, error) {
	mod, err := modulePath(root)
	if err != nil {
		return importPaths{}, err
	}
	return importPaths{module: mod}, nil
}

// join concatena o module com um subpath (ex.: "internal/repositories/foo").
func (p importPaths) join(sub string) string {
	if sub == "" {
		return p.module
	}
	return p.module + "/" + sub
}

// Subpaths (relativos ao module) usados na geração de código. Antes eram
// prefixados pelo module hardcoded; agora o prefixo vem de modulePath em runtime.
const (
	domainImportSubpath         = "internal/application/domain"
	servicesImportSubpath       = "internal/application/services"
	handlersImportSubpath       = "cmd/http/handlers"
	httperrImportSubpath        = "cmd/http/httperr"
	registryImportSubpath       = "pkg/registry"
	validatorImportSubpath      = "pkg/validator"
	filtersImportSubpath        = "internal/application/providers/filters"
	baseRepositoryImportSubpath = "internal/repositories/base_repository"
)
