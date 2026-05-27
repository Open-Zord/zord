package scaffold

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"github.com/spf13/cobra"
)

func newRepositoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repository",
		Short: "Operações sobre o adapter sqlx do domínio",
	}
	cmd.AddCommand(newRepositoryPortCmd())
	cmd.AddCommand(newRepositoryUnportCmd())
	cmd.AddCommand(newRepositoryCreateCmd())
	cmd.AddCommand(newRepositoryDeleteCmd())
	cmd.AddCommand(newRepositoryRegisterCmd())
	cmd.AddCommand(newRepositoryUnregisterCmd())
	return cmd
}

func newRepositoryPortCmd() *cobra.Command {
	var opts RepositoryPortOptions
	c := &cobra.Command{
		Use:   "port <Domain>",
		Short: "Adiciona ao arquivo do domínio os métodos e a interface Repository",
		Long: `Patcha o arquivo do domínio com:
  - métodos Schema/GetFilters/SoftDelete que satisfazem base_repository.Domain
  - setter SetFilters e campo filters *filters.Filters
  - interface Repository embedando base_repository.BaseRepository[<Domain>]
  - imports necessários

Com --multi-tenant adiciona também o campo client, SetClient e o prefix em Schema().

Falha se qualquer um dos elementos a gerar já existir.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RepositoryPort(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "atualizado:", path)
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	c.Flags().StringVar(&opts.Table, "table", "", "nome da tabela usado em Schema() (default: snake_case(Domain)+\"s\")")
	c.Flags().BoolVar(&opts.MultiTenant, "multi-tenant", false, "ativa o padrão client (campo + setter + prefix em Schema)")
	return c
}

func newRepositoryUnportCmd() *cobra.Command {
	var opts RepositoryUnportOptions
	c := &cobra.Command{
		Use:   "unport <Domain>",
		Short: "Remove do arquivo do domínio os métodos e a interface Repository",
		Long: `Operação inversa de ` + "`scaffold repository port`" + `. Edita o arquivo do
domínio em internal/application/domain/<snake>/<snake>.go removendo via AST:

  - métodos Schema/GetFilters/SoftDelete/SetFilters
  - campo não-exportado filters *filters.Filters
  - interface Repository
  - imports que ficarem sem uso

Multi-tenant é detectado automaticamente (campo client + SetClient). Estado
parcial (só um dos dois) é erro.

Falha se o domínio não estiver portado (any elemento ausente), sem mutar disco.

Se a interface Repository tiver métodos custom (além do embed BaseRepository),
ela é removida inteira e um aviso é emitido no stderr — o dev re-decide o que
fazer com os métodos descartados.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]

			// Detectar métodos custom ANTES da mutação pra emitir warning honesto.
			hasCustom := false
			if rel, abs := domainPaths(opts.Root, opts.Domain); abs != "" {
				_ = rel
				//nolint:gosec // G304: path derives from validated domain name
				if src, readErr := os.ReadFile(abs); readErr == nil {
					if f, parseErr := parser.ParseFile(token.NewFileSet(), abs, src, parser.ParseComments); parseErr == nil {
						hasCustom = repositoryHasCustomMethods(f)
					}
				}
			}

			path, err := RepositoryUnport(opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "editado:", path)
			if hasCustom {
				cmd.PrintErrln("aviso: interface Repository tinha métodos custom — foi removida inteira; tipos downstream que dependiam dela quebrarão a compilação")
			}
			return nil
		},
	}
	c.Flags().StringVar(&opts.Root, "root", "", "raiz do repositório (default: diretório atual)")
	return c
}

func newRepositoryCreateCmd() *cobra.Command {
	var opts RepositoryCreateOptions
	c := &cobra.Command{
		Use:   "create <Domain>",
		Short: "Cria o arquivo do repositório concreto que embeda BaseRepo[T]",
		Long: `Cria internal/repositories/<snake>/<snake>.go com pacote <snake>_repository,
struct embedando *base_repository.BaseRepo[<snake>.<Domain>] e o constructor
New<Domain>Repository(mysql *sqlx.DB).

Exige o arquivo do domínio existir e conter a struct <Domain>.
Falha se o arquivo do repositório já existe.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RepositoryCreate(opts)
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

func newRepositoryDeleteCmd() *cobra.Command {
	var opts RepositoryDeleteOptions
	c := &cobra.Command{
		Use:   "delete <Domain>",
		Short: "Apaga a pasta internal/repositories/<snake>/ (simétrico a create)",
		Long: `Remove internal/repositories/<snake>/ inteira via os.RemoveAll.

Falha se houver wire-up residual em bootstrap/repositories.go — rode
` + "`scaffold repository unregister <Domain>`" + ` antes. Falha se a pasta não
existir.

Não inspeciona services downstream que dependam do RegistryKey: o compile
error guia o próximo passo. Mesma postura do unregister.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RepositoryDelete(opts)
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

func newRepositoryRegisterCmd() *cobra.Command {
	var opts RegisterRepositoryOptions
	c := &cobra.Command{
		Use:   "register <Domain>",
		Short: "Registra o repository em bootstrap/repositories.go (DI wire-up)",
		Long: `Patcha bootstrap/repositories.go via AST adicionando:
  - import com alias <snake_domain sem underscores>repo do pacote do repositório
  - linha reg.Provide(<alias>.RegistryKey, <alias>.New<Domain>Repository(db))
    ao fim de registerRepositories(reg *registry.Registry)

Apenas a dependência universal (db) — exatamente o que o constructor gerado por
` + "`scaffold repository create`" + ` aceita. Quando o dev adiciona deps custom ao
constructor, a chamada quebra compilação até ser atualizada manualmente.

Exige o repository existir com const RegistryKey e func New<Domain>Repository.
Falha se o import OU a linha de Provide já existirem (idempotência).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := RegisterRepository(opts)
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

func newRepositoryUnregisterCmd() *cobra.Command {
	var opts UnregisterRepositoryOptions
	c := &cobra.Command{
		Use:   "unregister <Domain>",
		Short: "Remove a ligação do repository em bootstrap/repositories.go (DI wire-down)",
		Long: `Patcha bootstrap/repositories.go via AST removendo:
  - import com alias <snake_domain sem underscores>repo do pacote do repositório
  - linha reg.Provide(<alias>.RegistryKey, _) em registerRepositories

Simétrico a ` + "`scaffold repository register`" + ` (NAVE-72). Falha se o import OU
a linha de Provide não existirem (sem mutação parcial; idempotência).

Não apaga o pacote do repositório em internal/repositories/<snake>/ — desconecta
apenas do DI. Apagar o pacote é responsabilidade do dev (rm -rf à mão). Fluxo
natural de desmontagem: repository unregister → apagar pacote → ajustar
services downstream (compile error guia o próximo passo).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Domain = args[0]
			path, err := UnregisterRepository(opts)
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
