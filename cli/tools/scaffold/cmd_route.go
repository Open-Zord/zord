package scaffold

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRouteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Operações sobre o registro HTTP de rotas por domain",
	}
	cmd.AddCommand(newRouteCreateCmd())
	cmd.AddCommand(newRouteAddCmd())
	cmd.AddCommand(newRouteRemoveCmd())
	cmd.AddCommand(newRouteRegisterCmd())
	cmd.AddCommand(newRouteUnregisterCmd())
	cmd.AddCommand(newRouteDeleteCmd())
	return cmd
}

func newRouteCreateCmd() *cobra.Command {
	var opts RouteCreateOptions
	c := &cobra.Command{
		Use:   "create <Domain>",
		Short: "Cria o arquivo de rotas vazio do domain",
		Long: `Cria cmd/http/routes/<snake_domain>.go com:
  - package routes
  - struct <Pascal>Route{} (vazia — campos são adicionados por ` + "`route add`" + `)
  - constructor New<Pascal>Route() *<Pascal>Route
  - func (r *<Pascal>Route) DeclarePrivateRoutes(g *echo.Group, prefix string)
  - func (r *<Pascal>Route) DeclarePublicRoutes(g *echo.Group, prefix string)

Exige o arquivo do domínio existir (com a struct <Domain>) — rode antes:
    scaffold domain create <Domain>
Falha se o arquivo da Route já existe.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RouteCreate(opts)
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

func newRouteAddCmd() *cobra.Command {
	var opts RouteAddOptions
	c := &cobra.Command{
		Use:   "add <Domain> <Service> --method=<M> [--path=<p>] [--public]",
		Short: "Adiciona uma rota no arquivo do domain, apontando para o handler 1:1 do service",
		Long: `Patcha cmd/http/routes/<snake_domain>.go via AST adicionando:
  - campo <lowerCamel>Handler *<snake_service>.<Pascal>Handler na struct
  - parâmetro homônimo no construtor New<Pascal>Route (com atribuição no return)
  - linha g.<METHOD>("/"+prefix+"/<snake_domain>"+"<path>", r.<lowerCamel>Handler.Handle)
    em DeclarePrivateRoutes (default) ou DeclarePublicRoutes (--public)
  - import bare do handler em <module>/cmd/http/handlers/<snake_domain>/<snake_service>

Flags:
  --method (obrigatório) GET|POST|PUT|PATCH|DELETE
  --path   default /<kebab_service> (ex.: SelectOrg → /select-org)
  --public default false (sem flag → rota entra em DeclarePrivateRoutes)

Exige o service e o handler 1:1 existirem — rode antes:
    scaffold service create <Domain> <Service>
    scaffold handler create <Domain> <Service>
Falha se a rota já foi registrada (comparação por nome do campo do handler).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Service = args[1]
			path, err := RouteAdd(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "editado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&opts.Method, "method", "", "verbo HTTP: GET|POST|PUT|PATCH|DELETE (obrigatório)")
	c.Flags().StringVar(&opts.Path, "path", "", "path da rota relativo ao domain (default: /<kebab-service>)")
	c.Flags().BoolVar(&opts.Public, "public", false, "registra em DeclarePublicRoutes em vez de DeclarePrivateRoutes")
	_ = c.MarkFlagRequired("method")
	return c
}

func newRouteRemoveCmd() *cobra.Command {
	var opts RouteRemoveOptions
	c := &cobra.Command{
		Use:   "remove <Domain> <Service> [--force]",
		Short: "Remove a rota registrada por route add para o par (Domain, Service)",
		Long: `Desfaz scaffold route add. Patcha cmd/http/routes/<snake_domain>.go
via AST removendo, em uma única operação atômica:
  - campo <lowerCamel>Handler na struct <Pascal>Route
  - atribuição <lowerCamel>Handler: registry.Resolve[...] no CompositeLit
    do construtor New<Pascal>Route
  - linha g.<METHOD>(... r.<lowerCamel>Handler.Handle) em
    DeclarePrivateRoutes ou DeclarePublicRoutes (auto-detectado)
  - import do handler 1:1 se nenhuma outra rota no arquivo o referenciar

Flags:
  --force  remove o que existir dos 4 pontos e ignora os que faltarem
           (default: false → atomicidade estrita: todos ou nenhum)

Falha sempre (mesmo com --force) se o constructor New<Pascal>Route não tem
a assinatura canônica (reg *registry.Registry) — sem o shape canônico não
dá pra localizar o CompositeLit com segurança.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			opts.Service = args[1]
			path, err := RouteRemove(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "editado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().BoolVar(&opts.Force, "force", false, "remove parcialmente sem exigir todos os 4 pontos AST")
	return c
}

func newRouteRegisterCmd() *cobra.Command {
	var opts RouteRegisterOptions
	c := &cobra.Command{
		Use:   "register <Domain>",
		Short: "Registra a Route do domain em cmd/http/routes/declarable.go",
		Long: `Patcha cmd/http/routes/declarable.go via AST adicionando ao map retornado
por GetRoutes a entrada:

    "<snake_domain>": New<Pascal>Route(reg)

Sem mudanças em imports ou em linhas de Resolve — desde a NAVE-74 cada Route
resolve os próprios handlers internamente via *registry.Registry.

Exige a Route ter sido gerada pelo shape canônico (NAVE-74) — rode antes:
    scaffold route create <Domain>

Falha se:
  - a entrada já está presente em GetRoutes (idempotente — sem --force),
  - o constructor da Route foi hand-editado e tem parâmetros além de
    reg *registry.Registry. Nesse caso, registre o map manualmente.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RouteRegister(opts)
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

func newRouteUnregisterCmd() *cobra.Command {
	var opts RouteUnregisterOptions
	c := &cobra.Command{
		Use:   "unregister <Domain>",
		Short: "Remove a Route do domain de cmd/http/routes/declarable.go",
		Long: `Patcha cmd/http/routes/declarable.go via AST removendo do map retornado
por GetRoutes a entrada:

    "<snake_domain>": New<Pascal>Route(reg)

Operação inversa de ` + "`route register`" + ` (NAVE-65). NÃO apaga o arquivo
` + "`cmd/http/routes/<snake>.go`" + ` da Route — para isso, use ` + "`route delete`" + `
(simétrico a ` + "`route create`" + `).

Sem mudanças em imports — o ctor da Route não exige import externo além do
pacote ` + "`routes`" + ` interno.

Falha se:
  - a chave não está presente em GetRoutes (idempotência negativa: re-executar
    para o mesmo domain sempre falha após sucesso anterior).
  - declarable.go ou GetRoutes não tem o shape canônico.

Útil também para limpar entradas órfãs (chaves cujo arquivo da Route já foi
apagado à mão).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RouteUnregister(opts)
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

func newRouteDeleteCmd() *cobra.Command {
	var opts RouteDeleteOptions
	c := &cobra.Command{
		Use:   "delete <Domain>",
		Short: "Apaga o arquivo de rotas do domain (inverso de `route create`)",
		Long: `Apaga cmd/http/routes/<snake_domain>.go, agindo como inverso disciplinado
de ` + "`route create`" + `. Sem --force, sem --ignore-missing: o arquivo precisa
existir e o estado precisa estar limpo.

Guardas (todas obrigatórias, falham sem mutar disco):
  - Domain é PascalCase exportável
  - O arquivo cmd/http/routes/<snake_domain>.go existe
  - cmd/http/routes/declarable.go (se existir) não tem mais a chave
    "<snake_domain>" no map de GetRoutes — rode antes:
        # remover a entrada manualmente ou via route unregister
  - A struct <Pascal>Route está vazia (sem handlers anexados via route add).
    Se houver, o erro lista os campos e aponta edição manual

Idempotência inversa: re-executar pra o mesmo Domain sempre falha (arquivo
não existe na segunda chamada).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RouteDelete(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removido:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}
