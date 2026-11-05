// Package upgrader implementa `zord upgrade`: migra o runtime de um projeto
// gerado da versão com que foi criado até a versão do CLI, aplicando codemods
// (transformações de AST) versionados. Diferente de `zord package update`
// (file-sync + drift), o upgrade é migração ESTRUTURAL — necessário quando a
// mudança do core não é "troca o arquivo" e sim "ajuste este padrão onde quer
// que ele apareça", preservando as edições do dev.
package upgrader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// versionRelPath é o carimbo da versão do Zord com que o projeto está — escrito
// pelo `zord new` e avançado pelo `zord upgrade`.
const versionRelPath = ".zord/version"

// Migration é um passo de upgrade: leva o runtime do projeto ATÉ Version,
// aplicando um codemod. Apply recebe a raiz do projeto e o module path (p/
// codemods que reescrevem caminhos module-relativos).
type Migration struct {
	// Version é a release do Zord que esta migration entrega (ex.: "v1.1.0").
	Version string
	// Description é uma linha explicando o que o codemod faz.
	Description string
	// Apply é o codemod: transforma o projeto em root. Deve ser idempotente.
	Apply func(root, module string) error
}

// registered é o registro de produção das migrations, ordenado por Version.
// Começa VAZIO: v1.0.0 é o atual e não há migration real. Quando o core mudar,
// acrescente uma Migration aqui (e mantenha ordenado).
var registered []Migration

// Registered devolve as migrations de produção.
func Registered() []Migration { return registered }

// Pending devolve as migrations que faltam aplicar para levar um projeto de
// `from` (versão carimbada; vazia/não-release → baseline) até `target` (versão
// do CLI; vazia/não-release → ∞, i.e. aplica tudo), ordenadas por Version.
func Pending(from, target string, migs []Migration) []Migration {
	lo := from
	if !isRelease(lo) {
		lo = "v0.0.0"
	}
	hi := target
	if !isRelease(hi) {
		hi = "v999999.0.0"
	}
	var out []Migration
	for _, m := range migs {
		if compareRelease(lo, m.Version) < 0 && compareRelease(m.Version, hi) <= 0 {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return compareRelease(out[i].Version, out[j].Version) < 0 })
	return out
}

// ApplyPending aplica, em ordem, as migrations pendentes de `from` até `target`.
// Devolve as que foram aplicadas com sucesso; para no primeiro erro (retornando
// as já aplicadas + o erro) para o dev inspecionar o estado parcial.
func ApplyPending(root, module, from, target string, migs []Migration) ([]Migration, error) {
	pend := Pending(from, target, migs)
	applied := make([]Migration, 0, len(pend))
	for _, m := range pend {
		if err := m.Apply(root, module); err != nil {
			return applied, fmt.Errorf("migration %s: %w", m.Version, err)
		}
		applied = append(applied, m)
	}
	return applied, nil
}

// ReadVersion lê o carimbo .zord/version do projeto. Ausente → "" (sem erro):
// projetos gerados antes do carimbo caem no baseline.
func ReadVersion(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, versionRelPath)) //nolint:gosec // path derives from the repo root
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", versionRelPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteVersion grava o carimbo .zord/version, criando .zord/ se preciso.
func WriteVersion(root, version string) error {
	path := filepath.Join(root, versionRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("criar %s: %w", filepath.Dir(versionRelPath), err)
	}
	if err := os.WriteFile(path, []byte(version+"\n"), 0o600); err != nil {
		return fmt.Errorf("gravar %s: %w", versionRelPath, err)
	}
	return nil
}

// isRelease reporta se v é uma release semver limpa `vX.Y.Z` (sem prerelease).
func isRelease(v string) bool {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err != nil || n < 0 {
			return false
		}
	}
	return true
}

// compareRelease compara duas releases `vX.Y.Z` numericamente: <0, 0, >0.
// Entradas não-release são coagidas a 0.0.0 (não ocorrem para Migration.Version,
// que é sempre release; os bounds já são saneados por Pending).
func compareRelease(a, b string) int {
	av := parseRelease(a)
	bv := parseRelease(b)
	for i := range 3 {
		if av[i] != bv[i] {
			if av[i] < bv[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// parseRelease devolve [major, minor, patch] de `vX.Y.Z` (0,0,0 se inválido).
func parseRelease(v string) [3]int {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out
	}
	for i := range 3 {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return [3]int{}
		}
		out[i] = n
	}
	return out
}
