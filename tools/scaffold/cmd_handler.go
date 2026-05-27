package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHandlerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "handler",
		Short: "Operações sobre o adapter HTTP de um service",
	}
	cmd.AddCommand(newHandlerCreateCmd())
	cmd.AddCommand(newHandlerRegisterCmd())
	cmd.AddCommand(newHandlerUnregisterCmd())
	cmd.AddCommand(newHandlerDeleteCmd())
	return cmd
}

func newHandlerDeleteCmd() *cobra.Command {
	var opts HandlerDeleteOptions
	c := &cobra.Command{
		Use:   "delete <Domain> <Service>",
		Short: "Apaga a pasta do handler após validar que o wire-up e a rota já foram removidos",
		Long: `Apaga cmd/http/handlers/<snake_domain>/<snake_service>/ via os.RemoveAll
após validar, em ordem (falha sem mutar disco na primeira validação que falhar):

  1. Domain e Service são PascalCase exportáveis
  2. A pasta do handler existe
  3. Sem wire-up residual em bootstrap/handlers.go (import OU Provide)
  4. Sem rota residual em cmd/http/routes/<snake_domain>.go: campo
     <lowerCamel>Handler na struct da Route, import do pacote do handler,
     OU uso r.<lowerCamel>Handler.Handle em Declare*Routes

Bootstrap ausente conta como OK. Route file ausente conta como OK. Wire-up
vivo exige rodar scaffold handler unregister antes. Rota residual exige
edição manual do arquivo de rota (scaffold route remove ainda é backlog).

Fluxo natural de desmontagem:
    # edite cmd/http/routes/<snake_domain>.go removendo campo/import/uso do handler
    scaffold handler unregister <Domain> <Service>
    scaffold handler delete <Domain> <Service>
    scaffold service unregister <Domain> <Service>
    scaffold service delete <Domain> <Service>`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Service = args[1]
			path, err := HandlerDelete(opts)
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

func newHandlerCreateCmd() *cobra.Command {
	var opts HandlerCreateOptions
	c := &cobra.Command{
		Use:   "create <Domain> <Service>",
		Short: "Cria o handler HTTP 1:1 do service informado",
		Long: `Cria cmd/http/handlers/<snake_domain>/<snake_service>/handler.go com:
  - package <snake_service>
  - RegistryKey = "<lowerCamel>Handler"
  - struct <Pascal>Handler{ reg *registry.Registry }
  - constructor New<Pascal>Handler(reg *registry.Registry) *<Pascal>Handler
  - método único func (h *<Pascal>Handler) Handle(c echo.Context) error
    que resolve <service>.Service via registry.Resolve, binda em <service>.Input,
    chama svc.Execute e devolve o Output via c.JSON(http.StatusOK, out)

Exige o arquivo do domínio existir (com a struct <Domain>) e o service do use
case existir com const RegistryKey + func NewService — rode antes:
    scaffold domain create <Domain>
    scaffold service create <Domain> <Service>
Falha se a pasta do handler já existe.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Service = args[1]
			path, err := HandlerCreate(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "criado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newHandlerRegisterCmd() *cobra.Command {
	var opts RegisterHandlerOptions
	c := &cobra.Command{
		Use:   "register <Domain> <Service>",
		Short: "Registra o handler em bootstrap/handlers.go (DI wire-up)",
		Long: `Patcha bootstrap/handlers.go via AST adicionando:
  - import com alias <snake_domain sem _><snake_service sem _>handler do pacote
    do handler (ex.: Auth+Login → authloginhandler)
  - linha reg.Provide(<alias>.RegistryKey, <alias>.New<Service>Handler(reg))
    ao fim de registerHandlers(reg *registry.Registry)

Apenas a dependência universal (reg) — exatamente o que o constructor gerado por
` + "`scaffold handler create`" + ` aceita. Quando o dev adiciona deps custom ao
constructor, a chamada quebra compilação até ser atualizada manualmente.

Exige o handler existir com const RegistryKey e func New<Service>Handler — rode
antes:
    scaffold handler create <Domain> <Service>
Falha se o import OU a linha de Provide já existirem (idempotência).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Service = args[1]
			path, err := RegisterHandler(opts)
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

func newHandlerUnregisterCmd() *cobra.Command {
	var opts UnregisterHandlerOptions
	c := &cobra.Command{
		Use:   "unregister <Domain> <Service>",
		Short: "Remove o handler de bootstrap/handlers.go (desfaz a ligação no DI)",
		Long: `Edita bootstrap/handlers.go via AST removendo:
  - import com alias <snake_domain sem _><snake_service sem _>handler
    (ex.: Auth+Login → authloginhandler)
  - linha reg.Provide(<alias>.RegistryKey, _) em registerHandlers

Alias uniforme por construção (NAVE-73), sem detecção dual de formato. O segundo
argumento de Provide é ignorado: devs evoluem New<Service>Handler(reg) adicionando
deps custom, e o unregister precisa funcionar após essa evolução.

Não apaga o pacote do handler — só desconecta do DI. O fluxo natural de
desmontagem é:
    # edite cmd/http/routes/<snake_domain>.go removendo campo/import/uso do handler
    scaffold handler unregister <Domain> <Service>
    scaffold handler delete <Domain> <Service>
    scaffold service unregister <Domain> <Service>

Falha se o import OU a linha de Provide não existirem (idempotência sem mutação parcial).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Service = args[1]
			path, err := UnregisterHandler(opts)
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
