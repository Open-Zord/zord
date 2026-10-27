package mcp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// LockFileName é o path relativo (ao repo) do lock cross-process do MCP.
//
// Cada chamada de handler que toca arquivos compartilhados (derive_schema,
// repository_register, service_register, handler_register, route_register)
// pega lock exclusivo neste arquivo antes de chamar o scaffold, garantindo
// que duas instâncias concorrentes do servidor MCP (ou um servidor + uma
// invocação direta do binário scaffold) não escrevam no mesmo arquivo em
// paralelo. Tools que mutam arquivos por-domain (domain_create,
// repository_create, etc.) não precisam de lock — o conflito de escrita já é
// detectado pelo scaffold como "domain já existe".
const LockFileName = ".scaffold/lock"

// withRepoLock executa fn segurando lock exclusivo em <repo>/.scaffold/lock.
// A semântica é bloqueante: chamadas concorrentes esperam o lock liberar. O
// lock é cross-platform (gofrs/flock: flock no Unix, LockFileEx no Windows) e é
// liberado no defer; o arquivo permanece no FS (não tem GC — é só um sentinel
// de coordenação).
func withRepoLock(repo string, fn func() error) error {
	lockPath := filepath.Join(repo, LockFileName)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		return fmt.Errorf("criar diretório do lock %q: %w", filepath.Dir(lockPath), err)
	}

	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("lock exclusivo %q: %w", lockPath, err)
	}
	defer func() { _ = lock.Unlock() }()

	return fn()
}
