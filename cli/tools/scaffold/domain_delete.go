package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DomainDeleteOptions parametriza DomainDelete.
type DomainDeleteOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Organization").
	Domain string
	// Table sobrescreve o nome da tabela usado pra checar a sentinela no HCL.
	// Vazio = snake_case(Domain) + "s".
	Table string
	// SchemaPath sobrescreve o caminho do arquivo HCL. Vazio = DefaultSchemaPath.
	SchemaPath string
}

// DomainDelete apaga `internal/application/domain/<snake>/` se nenhuma camada
// downstream ainda referenciar o domínio. Inverso simétrico de DomainCreate
// (NAVE-56): falha se a pasta já não existir.
//
// Detecta dependências residuais (todas; nunca para no primeiro erro):
//   - internal/application/services/<snake>/...
//   - internal/repositories/<snake>/...
//   - cmd/http/handlers/<snake>/...
//   - cmd/http/routes/<snake>.go
//   - bloco `# scaffold:generated <table>` em schemas/schema.my.hcl
//
// Se qualquer dep existir, erro listando todas + comandos sugeridos pra limpar.
// Sem mutação parcial — `os.RemoveAll` só roda quando o ambiente está limpo.
// Retorna o caminho relativo da pasta removida.
func DomainDelete(opts DomainDeleteOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	snake := ToSnake(opts.Domain)
	table := opts.Table
	if table == "" {
		table = snake + "s"
	}
	schemaRel := opts.SchemaPath
	if schemaRel == "" {
		schemaRel = DefaultSchemaPath
	}

	relDir := filepath.Join(domainBasePath, snake)
	absDir := filepath.Join(root, relDir)

	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("domínio não existe: %s", relDir)
		}
		return "", fmt.Errorf("stat %s: %w", relDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s existe mas não é diretório", relDir)
	}

	deps, err := detectDomainDependencies(root, snake, table, schemaRel)
	if err != nil {
		return "", err
	}
	if len(deps) > 0 {
		return "", buildDependencyError(opts.Domain, deps)
	}

	if err := os.RemoveAll(absDir); err != nil {
		return "", fmt.Errorf("remover %s: %w", relDir, err)
	}
	return relDir, nil
}

// domainDependency descreve uma camada residual que impede o delete.
type domainDependency struct {
	// path é o caminho relativo do artefato encontrado.
	path string
	// suggest é o comando scaffold que limpa essa camada (forward refs ok).
	suggest string
}

// detectDomainDependencies acumula TODAS as deps residuais sem falhar no
// primeiro acerto. Erros de I/O são propagados de imediato (sintoma de
// problema na raiz, não de dep esperada).
func detectDomainDependencies(root, snake, table, schemaRel string) ([]domainDependency, error) {
	var deps []domainDependency

	checks := []struct {
		path    string // caminho relativo a checar
		suggest string
		isDir   bool
	}{
		{filepath.Join(servicesBasePath, snake), "scaffold service delete <Domain> <Verb>", true},
		{filepath.Join(repositoriesBasePath, snake), "scaffold repository delete <Domain>", true},
		{filepath.Join(handlersBasePath, snake), "scaffold handler delete <Domain> <Verb>", true},
		{filepath.Join(routesBasePath, snake+".go"), "scaffold route delete <Domain>", false},
	}
	for _, c := range checks {
		abs := filepath.Join(root, c.path)
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", c.path, err)
		}
		// Confere shape esperado (dir vs file). Algo do tipo errado é sinal de
		// estado corrompido — sinaliza pro dev investigar em vez de ignorar.
		if c.isDir && !info.IsDir() {
			return nil, fmt.Errorf("%s existe mas não é diretório", c.path)
		}
		if !c.isDir && info.IsDir() {
			return nil, fmt.Errorf("%s existe mas não é arquivo", c.path)
		}
		deps = append(deps, domainDependency{path: c.path, suggest: c.suggest})
	}

	// HCL sentinela — opcional: se o arquivo não existe, simplesmente pula.
	hasSentinel, err := schemaHasSentinel(root, schemaRel, table)
	if err != nil {
		return nil, err
	}
	if hasSentinel {
		deps = append(deps, domainDependency{
			path:    schemaRel + " (table " + table + ")",
			suggest: "scaffold derive schema --remove <Domain>",
		})
	}

	sort.Slice(deps, func(i, j int) bool { return deps[i].path < deps[j].path })
	return deps, nil
}

// schemaHasSentinel devolve true se o HCL contém um par
// `# scaffold:generated <table>` / `# scaffold:end <table>`. Reusa
// scanSentinels (NAVE-87) — qualquer inconsistência no arquivo é propagada.
// Arquivo ausente devolve (false, nil): se nunca houve schema, não há sentinela.
func schemaHasSentinel(root, schemaRel, table string) (bool, error) {
	abs := filepath.Join(root, schemaRel)
	raw, err := os.ReadFile(abs) //nolint:gosec // G304: path derives from validated opts + repo constants
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("ler %s: %w", schemaRel, err)
	}
	pairs, err := scanSentinels(raw)
	if err != nil {
		return false, fmt.Errorf("scan %s: %w", schemaRel, err)
	}
	_, ok := pairs[table]
	return ok, nil
}

// buildDependencyError formata o erro composto listando cada dep + comando
// sugerido. Ordem alfabética pra saída estável em CI/tests.
func buildDependencyError(domain string, deps []domainDependency) error {
	var b strings.Builder
	fmt.Fprintf(&b, "domínio %s tem %d dependência(s) residual(is); apague antes de domain delete:", domain, len(deps))
	for _, d := range deps {
		fmt.Fprintf(&b, "\n  - %s (rode: %s)", d.path, d.suggest)
	}
	return errors.New(b.String())
}
