package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Operações sobre services (use cases) do domínio",
	}
	cmd.AddCommand(newServiceCreateCmd())
	cmd.AddCommand(newServiceRegisterCmd())
	cmd.AddCommand(newServiceUnregisterCmd())
	cmd.AddCommand(newServiceDeleteCmd())
	return cmd
}

func newServiceCreateCmd() *cobra.Command {
	var opts ServiceCreateOptions
	c := &cobra.Command{
		Use:   "create <Domain> <Verb>",
		Short: "Cria o esqueleto de um service (usecase-per-folder)",
		Long: `Cria internal/application/services/<snake_domain>/<snake_verb>/service.go com:
  - package <snake_verb> e doc do pacote
  - RegistryKey = "<lowerCamelVerb>Service"
  - structs Input e Output vazias (dev preenche depois)
  - Service embedando services.BaseService
  - NewService(logger, idCreator) — sem injeção automática de ports do domínio
  - Execute(ctx, *Request) error e GetResponse() (*Response, error) — falhas via services.AppError

O foco é estrutural: o dev adiciona dependências do domínio à mão.
Exige o arquivo do domínio existir e conter a struct <Domain>.
Falha se a pasta do verbo já existe.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			paths, err := ServiceCreate(opts)
			if err != nil {
				return err
			}
			for _, p := range paths {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "criado:", p)
			}
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newServiceRegisterCmd() *cobra.Command {
	var opts RegisterServiceOptions
	c := &cobra.Command{
		Use:   "register <Domain> <Verb>",
		Short: "Registra o service em bootstrap/services.go (DI wire-up)",
		Long: `Patcha bootstrap/services.go via AST adicionando:
  - import do pacote do verbo (com alias <snake_domain>_<snake_verb> em colisão)
  - linha reg.Provide(<pkg>.RegistryKey, <pkg>.NewService(log, idC)) ao fim de
    registerServices(reg *registry.Registry)

Apenas as dependências universais (log, idC) — exatamente o que o constructor
gerado por ` + "`scaffold service create`" + ` aceita. Quando o dev adiciona ports ao
constructor, a chamada quebra compilação até ser atualizada manualmente.

Exige o service existir — rode antes:
    scaffold service create <Domain> <Verb>
Falha se o import OU a linha de Provide já existirem (idempotência).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			path, err := RegisterService(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "editado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newServiceDeleteCmd() *cobra.Command {
	var opts ServiceDeleteOptions
	c := &cobra.Command{
		Use:   "delete <Domain> <Verb>",
		Short: "Apaga a pasta do service após validar que o wire-up e o handler 1:1 já foram removidos",
		Long: `Apaga internal/application/services/<snake_domain>/<snake_verb>/ via os.RemoveAll
após validar, em ordem (falha sem mutar disco na primeira validação que falhar):

  1. Domain e Verb são PascalCase exportáveis
  2. A pasta do verbo existe
  3. Sem wire-up residual em bootstrap/services.go (import OU Provide)
  4. Sem handler 1:1 em cmd/http/handlers/<snake_domain>/<snake_verb>/

Bootstrap ausente conta como OK (paridade com service create). Wire-up vivo
exige rodar scaffold service unregister antes. Handler 1:1 vivo exige
apagar o handler (scaffold handler delete ou rm -rf) antes.

Fluxo natural de desmontagem:
    scaffold route unregister <Domain> <Verb>      # quando existir
    scaffold handler unregister <Domain> <Verb>    # quando existir
    scaffold service unregister <Domain> <Verb>
    scaffold service delete <Domain> <Verb>`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			path, err := ServiceDelete(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "apagado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newServiceUnregisterCmd() *cobra.Command {
	var opts UnregisterServiceOptions
	c := &cobra.Command{
		Use:   "unregister <Domain> <Verb>",
		Short: "Remove o service de bootstrap/services.go (desfaz a ligação no DI)",
		Long: `Edita bootstrap/services.go via AST removendo:
  - import do pacote do verbo (bare ou aliased <snake_domain>_<snake_verb>)
  - linha reg.Provide(<pkg>.RegistryKey, _) em registerServices

Detecta o formato real do import e usa o alias quando presente. O segundo
argumento de Provide é ignorado: devs evoluem NewService(log, idC) adicionando
ports do domínio, e o unregister precisa funcionar após essa evolução.

Não apaga o pacote do service — só desconecta do DI. O fluxo natural de
desmontagem é:
    scaffold service unregister <Domain> <Verb>
    scaffold service delete <Domain> <Verb>
    scaffold handler unregister <Domain> <Verb>    # quando existir
    scaffold route unregister <Domain> <Verb>      # quando existir

Falha se o import OU a linha de Provide não existirem (idempotência sem mutação parcial).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Verb = args[1]
			path, err := UnregisterService(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "editado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}
