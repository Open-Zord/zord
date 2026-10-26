package mcp

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RepoEnvVar é a env var que fornece o path do repositório alvo quando
// --repo não é passado.
const RepoEnvVar = "ZORD_REPO"

// resolveRepo extrai o path do repositório alvo dos args (flag --repo) ou
// da env var ZORD_REPO. Quando nenhum dos dois é fornecido, usa o diretório
// de trabalho atual (cwd). Valida que o path existe e é diretório.
//
// Retorna o path absoluto resolvido — todas as tools usam isso pra fazer
// prefix-guard nas chamadas que tocam arquivos.
func resolveRepo(args []string) (string, error) {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // erros vão pelo return, não pra stderr direto
	repoFlag := fs.String("repo", "", "path absoluto do repo alvo (default: $"+RepoEnvVar+" ou cwd)")
	if err := fs.Parse(args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	repo := *repoFlag
	if repo == "" {
		repo = os.Getenv(RepoEnvVar)
	}
	if repo == "" {
		// Default opensource: diretório de trabalho atual.
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolver cwd como repo default: %w", err)
		}
		repo = cwd
	}

	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", fmt.Errorf("resolver path %q: %w", repo, err)
	}

	// O path vem da config local do dev (flag --repo ou env). Servidor MCP
	// roda em stdio isolado e só acessa o repo declarado — não há superfície
	// de ataque pra path traversal aqui.
	info, err := os.Stat(abs) //nolint:gosec // G304/G703: path controlado pelo operador

	if err != nil {
		return "", fmt.Errorf("acessar repo %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", errors.New("repo path não é diretório: " + abs)
	}

	return abs, nil
}

// effectiveRepo resolve o path do repo alvo de uma chamada de tool. Quando
// override está vazio devolve defaultRepo (já validado por resolveRepo no
// startup). Quando override é não vazio: resolve pra absoluto, exige que seja
// diretório existente e que contenha um go.mod na raiz (sanity check mínimo de
// "isso é um repo Go").
//
// Esta função é chamada por todos os handlers de tools de escrita para
// suportar redirecionamento per-call do scaffold para a worktree apropriada.
func effectiveRepo(override, defaultRepo string) (string, error) {
	if override == "" {
		return defaultRepo, nil
	}

	abs, err := filepath.Abs(override)
	if err != nil {
		return "", fmt.Errorf("resolver path %q: %w", override, err)
	}

	// Path vem do input da tool (controlado pelo cliente MCP — dev local).
	// Mesma justificativa de resolveRepo: superfície de ataque inexistente.
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("acessar repo %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repo path não é diretório: %s", abs)
	}

	goMod := filepath.Join(abs, "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		return "", fmt.Errorf("repo %q não tem go.mod: %w", abs, err)
	}

	return abs, nil
}
