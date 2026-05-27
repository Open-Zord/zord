// Package scaffold (área service delete) fecha o ciclo de desmontagem de um
// service iniciado por `service unregister` (NAVE-88). `service delete`
// (NAVE-97) apaga a pasta `internal/application/services/<snake_domain>/
// <snake_verb>/` com guardas que recusam a operação enquanto o ecossistema
// do verbo ainda tem dependências vivas.
package scaffold

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

// ServiceDeleteOptions parametriza ServiceDelete.
type ServiceDeleteOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "OrgMembership").
	Domain string
	// Verb é o nome do verbo do use case em PascalCase (ex.: "Login",
	// "SelectOrg").
	Verb string
}

// ServiceDelete apaga `internal/application/services/<snake_domain>/<snake_verb>/`
// via `os.RemoveAll` após validar, em ordem, que:
//
//  1. Domain e Verb são PascalCase exportáveis.
//  2. A pasta do verbo existe.
//  3. Não há wire-up residual em `bootstrap/services.go` (import OU Provide).
//     Bootstrap ausente conta como OK; bootstrap presente sem
//     `registerServices` também conta como OK (não há wire-up possível).
//  4. Não existe handler 1:1 em `cmd/http/handlers/<snake_domain>/<snake_verb>/`.
//
// Falha sem mutar disco na primeira validação que falhar. A ordem é
// pasta → wire-up → handler: pasta primeiro porque é a invariante mais barata
// e dá a mensagem mais útil ("nada pra apagar"); wire-up antes do handler
// porque é o acoplamento mais perigoso (`Resolve` em runtime panica enquanto
// handler órfão só quebra build).
//
// Retorna o caminho relativo da pasta apagada.
func ServiceDelete(opts ServiceDeleteOptions) (string, error) {
	plan, err := planService(RegisterServiceOptions(opts))
	if err != nil {
		return "", err
	}

	relDir := filepath.Join(servicesBasePath, plan.snakeDomain, plan.snakeVerb)
	absDir := filepath.Join(plan.root, relDir)

	if _, err := os.Stat(absDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("service %s não existe", relDir)
		}
		return "", fmt.Errorf("stat %s: %w", absDir, err)
	}

	if err := assertNoServiceWireUp(plan); err != nil {
		return "", err
	}

	relHandlerDir := filepath.Join(handlersBasePath, plan.snakeDomain, plan.snakeVerb)
	absHandlerDir := filepath.Join(plan.root, relHandlerDir)
	if _, err := os.Stat(absHandlerDir); err == nil {
		return "", fmt.Errorf("handler 1:1 ainda existe em %s — apague o handler antes (scaffold handler delete ou rm -rf)", relHandlerDir)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", absHandlerDir, err)
	}

	if err := os.RemoveAll(absDir); err != nil {
		return "", fmt.Errorf("remover %s: %w", relDir, err)
	}
	return relDir, nil
}

// assertNoServiceWireUp parseia `bootstrap/services.go` (se existir) e devolve
// erro se ainda houver import OU linha de Provide associada ao verbo. Reusa
// `findImportSpec`, `importIdent` e `findProvideStmt` definidos em
// register_service.go/unregister_service.go.
//
// Casos OK (retornam nil):
//   - `bootstrap/services.go` ausente — repo ainda sem bootstrap.
//   - Arquivo presente mas sem `registerServices` — não há wire-up possível.
//   - Arquivo presente, função presente, mas nem import nem Provide referem
//     o verbo.
func assertNoServiceWireUp(plan servicePlan) error {
	src, err := os.ReadFile(plan.absFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ler %s: %w", plan.relFile, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, plan.absFile, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", plan.relFile, err)
	}

	if imp := findImportSpec(file, plan.importPath); imp != nil {
		_ = importIdent(imp)
		return fmt.Errorf("em %s: import %q ainda presente — rode scaffold service unregister %s %s antes", plan.relFile, plan.importPath, plan.snakeDomain, plan.snakeVerb)
	}

	registerFn, ferr := findFreeFunc(file, registerServicesFunc)
	if ferr != nil {
		return nil //nolint:nilerr // ausência da função é intencionalmente OK
	}

	// Verifica Provide tanto pelo identificador bare (snake_verb) quanto pelo
	// alias usado quando há colisão (<snake_domain>_<snake_verb>). Cobre o
	// caso patológico de arquivo com Provide mas sem o import equivalente
	// (estado inconsistente que ainda precisa ser limpo antes do delete).
	//
	// Importante: o identificador `<pkgIdent>` em `reg.Provide(<pkgIdent>.X, _)`
	// é resolvido pelo *import* atualmente no arquivo. Se outro import (de path
	// distinto) já expõe esse mesmo identificador — ex.: `invoice/get` (bare)
	// vs `widget/get` (aliased) — o Provide bare pertence ao OUTRO pacote, não
	// ao verbo que estamos deletando. Nesse caso o Provide encontrado não é
	// resíduo nosso e a guarda não deve disparar (falso positivo).
	//
	// `importIdentTaken` responde "esse identificador já está sendo usado por
	// algum import no arquivo?" — quando a resposta é true, qualquer Provide
	// com esse pkgIdent é wire-up de outro service, não nosso.
	candidates := []string{plan.snakeVerb, plan.snakeDomain + "_" + plan.snakeVerb}
	for _, pkgIdent := range candidates {
		if findProvideStmt(registerFn, pkgIdent) == nil {
			continue
		}
		if importIdentTaken(file, pkgIdent) {
			// Provide bate por nome mas o identificador pertence a outro
			// import vivo no arquivo — não é resíduo do verbo deletado.
			continue
		}
		return fmt.Errorf("em %s: reg.Provide(%s.RegistryKey, ...) ainda presente — rode scaffold service unregister %s %s antes", plan.relFile, pkgIdent, plan.snakeDomain, plan.snakeVerb)
	}
	return nil
}
