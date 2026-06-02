package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Operações sobre entrypoints de workers de fila (bootstrap/<name>/ + cmd/<name>/)",
	}
	cmd.AddCommand(newQueueCreateCmd())
	return cmd
}

func newQueueCreateCmd() *cobra.Command {
	var opts QueueCreateOptions
	var driver string
	c := &cobra.Command{
		Use:   "create <name>",
		Short: "Cria o esqueleto de um novo entrypoint de worker de fila (driver omniq)",
		Long: `Gera o esqueleto de um entrypoint de worker de fila — 6 arquivos em
bootstrap/<name>/ (setup, configs, pkg, repositories, services, worker) +
cmd/<name>/main.go. Por ora só o driver "omniq" (github.com/not-empty/omniq-go)
é suportado; o flag --driver fica como gancho pra futuros backends.

Filosofia:
  - 1 entrypoint == 1 fila == 1 worker. Não há lista de workers no bootstrap;
    o handler é construído por NewHandler(reg) em worker.go.
  - Nome da fila Redis vem do env OMNIQ_QUEUE em runtime — o <name> é só
    convenção de pacote Go.
  - repositories.go e services.go saem vazios; os scaffolds existentes vão
    apender registrations aqui quando virarem entrypoint-aware (--entrypoint).

main.go gerado chama <name>boot.Run(), que internamente faz Setup() + client.Consume.
Crash do consume loop derruba o processo; orquestrador externo reinicia.

<name> é o nome do pacote Go em lowercase (ex.: "worker", "billing_worker").
Validações:
  - Identificador Go válido (^[a-z][a-z0-9_]*$), não keyword.
  - Driver suportado (omniq por default).
  - Nenhum dos diretórios alvo (cmd/<name>/ ou bootstrap/<name>/) existe.

Depois de rodar, lembre de "go mod tidy" pra trazer github.com/not-empty/omniq-go
ao go.sum do repo alvo.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			opts.Driver = QueueDriver(driver)
			created, err := QueueCreate(opts)
			if err != nil {
				return err
			}
			for _, p := range created {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "criado:", p)
			}
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&driver, "driver", string(QueueDriverOmniq), "driver da fila (suportado: omniq)")
	return c
}
