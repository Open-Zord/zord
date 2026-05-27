package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	arch "github.com/Open-Zord/zord/tools/arch_analyser"
)

// archAnalyzeInput é o argumento da tool arch_analyze. Aceita override per-call
// do repo, consistente com as tools de scaffold.
type archAnalyzeInput struct {
	Repo string `json:"repo,omitempty" jsonschema:"path absoluto do repo alvo (default: --repo do startup)"`
}

// archAnalyseOutput é estruturado: a tool retorna o status agregado e a lista
// de violações por validador. Sucesso = OK true + Violations vazio.
type archAnalyseOutput struct {
	OK         bool       `json:"ok"`
	Violations []archCase `json:"violations,omitempty"`
}

type archCase struct {
	Validator string `json:"validator"`
	Error     string `json:"error"`
}

// archValidators casa cada nome (humano) com a função de validação. Mantém
// a ordem da CLI arch-analyze pro output ser determinístico.
var archValidators = []struct {
	name string
	fn   func(root string) error
}{
	{"ValidateImports", arch.ValidateImports},
	{"ValidateNewServiceParams", arch.ValidateNewServiceParams},
	{"ValidateDbQueriesInRepositories", arch.ValidateDbQueriesInRepositories},
	{"ValidateLayerDependencies", arch.ValidateLayerDependencies},
	{"ValidateNoCircularDependencies", arch.ValidateNoCircularDependencies},
	{"ValidateContextUsage", arch.ValidateContextUsage},
	{"ValidateProvidersArePure", arch.ValidateProvidersArePure},
	{"ValidateRepositoryInterfacesImplemented", arch.ValidateRepositoryInterfacesImplemented},
	{"ValidateNoGlobalVars", arch.ValidateNoGlobalVars},
	{"ValidateOrchestrationOnlyInServices", arch.ValidateOrchestrationOnlyInServices},
	{"ValidateNamingAndLocation", arch.ValidateNamingAndLocation},
	{"ValidateExternalPackagesUsage", arch.ValidateExternalPackagesUsage},
	{"ValidateReflectionUsage", arch.ValidateReflectionUsage},
	{"ValidatePkgNoInternalImports", arch.ValidatePkgNoInternalImports},
	{"ValidateDomainNaming", arch.ValidateDomainNaming},
	{"ValidateTypeAssertions", arch.ValidateTypeAssertions},
	{"ValidateNoHandlerCrossImports", arch.ValidateNoHandlerCrossImports},
	{"ValidateScaffoldLayout", arch.ValidateScaffoldLayout},
	{"ValidateNoOrphanPackages", arch.ValidateNoOrphanPackages},
}

func registerArch(s *mcpsdk.Server, repo string) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "arch_analyze",
		Description: "Executa todas as validações de arquitetura (camadas, imports, naming, providers, etc.) sobre o repositório alvo e retorna lista estruturada de violações.",
		Annotations: readOnlyAnnotations("Arch analyze"),
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in archAnalyzeInput) (*mcpsdk.CallToolResult, archAnalyseOutput, error) {
		target, err := effectiveRepo(in.Repo, repo)
		if err != nil {
			return nil, archAnalyseOutput{}, fmt.Errorf("arch_analyze: %w", err)
		}
		var violations []archCase
		for _, v := range archValidators {
			if err := v.fn(target); err != nil {
				violations = append(violations, archCase{
					Validator: v.name,
					Error:     strings.TrimSpace(err.Error()),
				})
			}
		}
		return nil, archAnalyseOutput{
			OK:         len(violations) == 0,
			Violations: violations,
		}, nil
	})
}
