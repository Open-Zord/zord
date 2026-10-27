// Package archanalyse expõe o comando `zord arch-analyse`, que roda o gate de
// arquitetura sobre o diretório atual.
package archanalyse

import (
	"fmt"
	"os"

	arch "github.com/Open-Zord/zord/cli/tools/archanalyser"

	"github.com/spf13/cobra"
)

// validações é a sequência do gate; cada entrada é um validador que recebe a
// raiz do projeto e devolve erro. Adicionar regra = uma linha aqui.
var validações = []struct {
	nome string
	run  func(string) error
}{
	{"importações", arch.ValidateImports},
	{"parâmetros de NewService", arch.ValidateNewServiceParams},
	{"queries no banco", arch.ValidateDbQueriesInRepositories},
	{"dependências entre camadas", arch.ValidateLayerDependencies},
	{"circularidade de dependências", arch.ValidateNoCircularDependencies},
	{"uso de context", arch.ValidateContextUsage},
	{"pureza dos providers", arch.ValidateProvidersArePure},
	{"implementação de interfaces de repositório", arch.ValidateRepositoryInterfacesImplemented},
	{"variáveis globais mutáveis", arch.ValidateNoGlobalVars},
	{"orquestração", arch.ValidateOrchestrationOnlyInServices},
	{"nomenclatura e localização", arch.ValidateNamingAndLocation},
	{"uso de pacotes externos", arch.ValidateExternalPackagesUsage},
	{"uso de reflection", arch.ValidateReflectionUsage},
	{"type assertions", arch.ValidateTypeAssertions},
	{"pacotes órfãos", arch.ValidateNoOrphanPackages},
	{"imports de internal em pkg", arch.ValidatePkgNoInternalImports},
	{"nomenclatura de domínios", arch.ValidateDomainNaming},
	{"cross-imports entre handlers", arch.ValidateNoHandlerCrossImports},
	{"layout do scaffold", arch.ValidateScaffoldLayout},
}

// Commands devolve o(s) comando(s) deste grupo para o root do CLI.
func Commands() []*cobra.Command {
	return []*cobra.Command{
		{
			Use:   "arch-analyse",
			Short: "Analisa as importações dos domínios para garantir regras de arquitetura",
			RunE: func(_ *cobra.Command, _ []string) error {
				root, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("obter diretório de trabalho: %w", err)
				}
				for _, v := range validações {
					if err := v.run(root); err != nil {
						return fmt.Errorf("validação de %s: %w", v.nome, err)
					}
					fmt.Printf("Validação de %s concluída com sucesso!\n", v.nome)
				}
				return nil
			},
		},
	}
}
