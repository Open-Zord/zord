package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
)

// RouteUnregisterOptions parametriza RouteUnregister.
type RouteUnregisterOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Namespace", "UsageRecord").
	// Determina a chave do map a remover (`<snake>`).
	Domain string
}

// RouteUnregister patcha `cmd/http/routes/declarable.go` removendo a entrada
//
//	"<snake_domain>": New<Pascal>Route(reg)
//
// do map literal retornado por `GetRoutes`. Retorna o caminho relativo do
// arquivo editado.
//
// Operação inversa de RouteRegister (NAVE-65). Não é inversa de RouteCreate —
// o arquivo `cmd/http/routes/<snake>.go` da Route NÃO é apagado. Permite
// também limpar entradas órfãs (chaves cujo arquivo da Route já foi apagado
// à mão).
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Domain é PascalCase exportável.
//   - `cmd/http/routes/declarable.go` existe e tem `func GetRoutes(...)`
//     terminando em `return map[string]Declarable{...}`.
//   - A chave `"<snake_domain>"` está presente no map (idempotência negativa:
//     re-executar para o mesmo domain sempre falha).
//
// Imports em `declarable.go` ficam intocados — o ctor da Route não exige
// import externo além do pacote `routes` interno.
func RouteUnregister(opts RouteUnregisterOptions) (string, error) {
	plan, err := planUnregister(opts)
	if err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	src, err := os.ReadFile(plan.absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", plan.relFile, err)
	}
	file, err := parser.ParseFile(fset, plan.absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", plan.relFile, err)
	}

	fn, err := findFreeFuncDecl(file, getRoutesFunc)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	mapLit, err := findRoutesMapLit(fn)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", plan.relFile, err)
	}
	if !hasRouteEntry(mapLit, plan.snakeDomain) {
		return "", fmt.Errorf("em %s: entrada %q não está presente em GetRoutes", plan.relFile, plan.snakeDomain)
	}

	if !removeRouteEntry(mapLit, plan.snakeDomain) {
		// Defesa: hasRouteEntry e removeRouteEntry usam o mesmo critério;
		// chegar aqui significa inconsistência interna.
		return "", fmt.Errorf("em %s: falha removendo entrada %q do map", plan.relFile, plan.snakeDomain)
	}

	// Re-stamping é necessário pelo mesmo motivo do RouteRegister: sem
	// reservar uma linha por KV remanescente, o go/printer pode emitir
	// layout inconsistente quando a chave removida era a mais longa. O
	// Rbrace do body de GetRoutes também é re-stampado pra evitar linha em
	// branco residual entre o `}` do CompositeLit e o `}` da função.
	padder := NewLinePadder(fset, "scaffold-route-unregister")
	stampMapLit(padder, mapLit)
	fn.Body.Rbrace = padder.Take()

	if err := writeFile(plan.absFile, fset, file); err != nil {
		return "", err
	}
	return plan.relFile, nil
}

// unregisterPlan agrupa nomes derivados e caminhos pra evitar repetir a
// derivação nos passos de RouteUnregister.
type unregisterPlan struct {
	root        string
	relFile     string
	absFile     string
	snakeDomain string
}

func planUnregister(opts RouteUnregisterOptions) (unregisterPlan, error) {
	var plan unregisterPlan
	if !IsValidExportedIdent(opts.Domain) {
		return plan, fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	plan.root = opts.Root
	if plan.root == "" {
		plan.root = "."
	}
	plan.snakeDomain = ToSnake(opts.Domain)
	plan.relFile = declarableRelPath
	plan.absFile = filepath.Join(plan.root, plan.relFile)
	return plan, nil
}

// removeRouteEntry remove do CompositeLit o KeyValueExpr cuja chave é a
// string literal `"<snake_domain>"`. Retorna true se removeu, false caso
// contrário. Espelha o critério de hasRouteEntry — chamar hasRouteEntry
// antes (no caminho "valida primeiro, muta depois") garante que esta função
// só executa quando a entrada existe.
func removeRouteEntry(cl *ast.CompositeLit, snakeDomain string) bool {
	quoted := strconv.Quote(snakeDomain)
	for i, e := range cl.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		lit, ok := kv.Key.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if lit.Value == quoted {
			cl.Elts = append(cl.Elts[:i], cl.Elts[i+1:]...)
			return true
		}
	}
	return false
}
