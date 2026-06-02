// Package scaffold (área queue) cria o esqueleto de um entrypoint de worker
// de fila. Hoje só o driver `omniq` (github.com/not-empty/omniq-go) está
// implementado, mas a estrutura suporta novos drivers (sqs, rabbitmq, kafka,
// nats, ...) sem mudar este arquivo: cada driver mora num
// `queue_<driver>.go` autocontido com seus próprios builders e se registra
// em `queueDrivers` abaixo.
//
// Filosofia: 1 entrypoint == 1 fila == 1 worker. O nome da fila vem do env
// scoped por entrypoint (ex.: `OMNIQ_BILLING_WORKER_QUEUE`) em runtime — o
// nome do entrypoint Go (`billing_worker`) é só convenção de pacote. Crash
// do consume loop derruba o processo; orquestrador externo (k8s/systemd)
// reinicia.
//
// Layout dos arquivos gerados espelha `bootstrap/http/` por simetria
// (`setup`, `configs`, `pkg`, `repositories`, `services`, `worker`,
// `cmd/<name>/main.go`) + um `.env.<name>` versionado na raiz com defaults
// de dev. `repositories.go`/`services.go` saem com corpo vazio —
// driver-agnósticos, vivem em queue_common.go pra serem reusados quando
// novos drivers chegarem.
//
// Geração via AST puro (mesma regra do resto do scaffold).
package scaffold

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// QueueDriver enumera os drivers de fila suportados. Adicionar um novo
// driver: criar `queue_<driver>.go` com a função builder e adicionar uma
// entrada em `queueDrivers` abaixo.
type QueueDriver string

const (
	// QueueDriverOmniq gera o entrypoint contra github.com/not-empty/omniq-go.
	QueueDriverOmniq QueueDriver = "omniq"
)

// queueFile é o par (caminho relativo, source bytes) que cada builder
// retorna. Compartilhado entre todos os drivers via queue_common.go.
type queueFile struct {
	relPath string
	src     []byte
}

// queueFileBuilder é a assinatura que cada driver implementa em seu
// `queue_<driver>.go`. Recebe o nome do entrypoint (snake_case lowercase) e
// o resolver de paths de import; devolve a lista ordenada de arquivos a
// criar (a ordem é mantida no retorno do QueueCreate pra reporting estável).
type queueFileBuilder func(name string, imp importPaths) ([]queueFile, error)

// queueDrivers é o registro dos drivers suportados. Cada driver mora num
// arquivo `queue_<driver>.go` separado e se "registra" plugando sua função
// builder aqui. A função buildFor() faz o lookup; QueueCreate valida o
// driver contra essa mesma tabela.
var queueDrivers = map[QueueDriver]queueFileBuilder{
	QueueDriverOmniq: buildOmniqQueueFiles,
}

// QueueCreateOptions parametriza QueueCreate.
type QueueCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Name é o nome do entrypoint em lowercase (ex.: "worker",
	// "billing_worker"). Vira o pacote Go em bootstrap/<name>/ e o segmento
	// em cmd/<name>/. Não é o nome da fila no broker — esse vem do env
	// scoped (ex.: OMNIQ_BILLING_WORKER_QUEUE) em runtime.
	Name string
	// Driver seleciona o backend da fila. Vazio default pra QueueDriverOmniq.
	Driver QueueDriver
}

// QueueCreate gera o esqueleto completo do entrypoint de worker. A lista de
// arquivos retornada é determinada pelo driver — cada driver decide o que
// produz. A ordem do retorno é a ordem canônica do driver (bootstrap →
// cmd → .env) e é estável entre chamadas.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Name é identificador Go válido em lowercase (não keyword).
//   - Driver é um QueueDriver registrado em queueDrivers.
//   - Nenhum dos diretórios alvo (cmd/<name>/ ou bootstrap/<name>/) existe.
func QueueCreate(opts QueueCreateOptions) ([]string, error) {
	if !IsValidPackageIdent(opts.Name) {
		return nil, fmt.Errorf("nome de entrypoint inválido (esperado lowercase Go ident, não keyword): %q", opts.Name)
	}
	driver := opts.Driver
	if driver == "" {
		driver = QueueDriverOmniq
	}
	build, ok := queueDrivers[driver]
	if !ok {
		return nil, fmt.Errorf("driver de fila não suportado: %q (suportados: %s)", driver, supportedDriversList())
	}

	root := opts.Root
	if root == "" {
		root = "."
	}

	relBootstrapDir := filepath.Join(bootstrapBasePath, opts.Name)
	relCmdDir := filepath.Join(cmdBasePath, opts.Name)
	absBootstrapDir := filepath.Join(root, relBootstrapDir)
	absCmdDir := filepath.Join(root, relCmdDir)

	if err := assertDirAbsent(absBootstrapDir, relBootstrapDir); err != nil {
		return nil, err
	}
	if err := assertDirAbsent(absCmdDir, relCmdDir); err != nil {
		return nil, err
	}

	imp, err := newImportPaths(root)
	if err != nil {
		return nil, err
	}

	files, err := build(opts.Name, imp)
	if err != nil {
		return nil, err
	}

	// Mutação determinística: na ordem que o driver devolveu. Falha parcial é
	// reportada (sem rollback), o dev resolve manualmente — mesma postura do
	// entrypoint_create.
	created := make([]string, 0, len(files))
	for _, f := range files {
		absFile := filepath.Join(root, f.relPath)
		absDir := filepath.Dir(absFile)
		if err := writeNewFile(absDir, absFile, f.src); err != nil {
			return nil, err
		}
		created = append(created, f.relPath)
	}
	return created, nil
}

// supportedDriversList devolve os drivers registrados em ordem alfabética,
// formatados pra mensagem de erro. Ordenação garante saída estável
// independente da iteração do map.
func supportedDriversList() string {
	if len(queueDrivers) == 0 {
		return "(nenhum)"
	}
	names := make([]string, 0, len(queueDrivers))
	for d := range queueDrivers {
		names = append(names, `"`+string(d)+`"`)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
