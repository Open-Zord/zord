package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newEntrypointCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entrypoint",
		Short: "Operações sobre entrypoints do monorepo (cmd/<name>/ + bootstrap/<name>/)",
	}
	cmd.AddCommand(newEntrypointCreateCmd())
	return cmd
}

func newEntrypointCreateCmd() *cobra.Command {
	var opts EntrypointCreateOptions
	c := &cobra.Command{
		Use:   "create <name>",
		Short: "Cria o esqueleto mínimo de um novo entrypoint",
		Long: `Gera o esqueleto mínimo de um novo entrypoint do monorepo, com um arquivo
por camada:

  - bootstrap/<name>/setup.go: package <name> com func Setup() *registry.Registry
    que apenas instancia o registry. Extensão (configs, primitives, repositories,
    services, handlers, etc.) fica a cargo de scaffolds futuros específicos por
    tipo de entrypoint (grpc, graphql, kubernetes operator, redis_queue, ...).
  - cmd/<name>/main.go: package main que chama Setup() e descarta o registry com
    "_ = reg". Esqueleto compila e funciona como ponto de partida pro dev
    implementar o runtime do entrypoint.

<name> é o nome do pacote Go em lowercase (ex.: "cli", "grpc", "redis_queue").
Validações:
  - Identificador Go válido (^[a-z][a-z0-9_]*$), não keyword.
  - Nenhum dos diretórios alvo (cmd/<name>/ ou bootstrap/<name>/) existe.

Não toca em bootstrap/http/ nem em outros entrypoints já existentes — cada
entrypoint vive isolado em seu próprio subpacote.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			bp, cp, err := EntrypointCreate(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "criado:", bp)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "criado:", cp)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}
