package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// LockFileName é o path relativo (ao repo) do lock cross-process do MCP.
//
// Cada chamada de handler que toca arquivos compartilhados (derive_schema,
// repository_register, service_register, handler_register, route_register)
// pega flock exclusivo neste arquivo antes de chamar o scaffold, garantindo
// que duas instâncias concorrentes do servidor MCP (ou um servidor + uma
// invocação direta do binário scaffold) não escrevam no mesmo arquivo em
// paralelo. Tools que mutam arquivos por-domain (domain_create,
// repository_create, etc.) não precisam de lock — o conflito de escrita já é
// detectado pelo scaffold como "domain já existe".
const LockFileName = ".scaffold/lock"

// withRepoLock executa fn segurando flock exclusivo em <repo>/.scaffold/lock.
// A semântica é bloqueante: chamadas concorrentes esperam o flock liberar.
// Lock é liberado pelo close do fd no defer; o arquivo permanece no FS
// (não tem GC — é só um sentinel de coordenação).
func withRepoLock(repo string, fn func() error) error {
	lockPath := filepath.Join(repo, LockFileName)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		return fmt.Errorf("criar diretório do lock %q: %w", filepath.Dir(lockPath), err)
	}

	// O_CREATE garante o sentinel mesmo no primeiro uso; O_RDWR é o suficiente
	// pra flock exclusivo (LOCK_EX não exige permissão de escrita no FS, mas
	// alguns kernels antigos reclamam de LOCK_EX em O_RDONLY).
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // path resolvido a partir do repo controlado
	if err != nil {
		return fmt.Errorf("abrir lock %q: %w", lockPath, err)
	}
	defer func() { _ = f.Close() }()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil { //nolint:gosec // G115: f.Fd() devolve uintptr de fd válido (sempre cabe em int em Linux)
		return fmt.Errorf("flock LOCK_EX %q: %w", lockPath, err)
	}
	// Flock é liberado automaticamente pelo close do fd (defer acima); não
	// precisamos chamar LOCK_UN explicitamente.

	return fn()
}
