package scaffold

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// RouteDeleteOptions parametriza RouteDelete.
type RouteDeleteOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "Auth", "UsageRecord").
	// Determina o arquivo a apagar (cmd/http/routes/<snake_domain>.go) e
	// o nome da struct cujo estado é verificado (<Pascal>Route).
	Domain string
}

// RouteDelete apaga cmd/http/routes/<snake_domain>.go, agindo como inverso
// disciplinado de RouteCreate. Falha sem mutar disco se qualquer guarda
// estoura. Retorna o caminho relativo do arquivo apagado.
//
// Idempotência inversa: o arquivo precisa existir. Re-executar RouteDelete
// no mesmo Domain sempre falha — não há --force nem --ignore-missing.
//
// Guardas (todas obrigatórias, falham sem remover nada do disco):
//
//   - Domain é PascalCase exportável.
//   - O arquivo cmd/http/routes/<snake>.go existe.
//   - Sem entrada residual em cmd/http/routes/declarable.go: se o arquivo
//     existir, o map literal retornado por GetRoutes não pode conter a
//     chave "<snake_domain>". Se declarable.go não existir, passa por
//     vacuidade (repos novos ou em scaffold).
//   - Struct <Pascal>Route vazia: Fields.List precisa estar vazio. Se
//     houver handlers anexados via `route add`, a remoção é manual — o erro
//     lista os campos.
//
// A guarda do declarable reusa findFreeFuncDecl, findRoutesMapLit e
// hasRouteEntry de route_register.go (single source of truth da validação
// do registro de rotas).
func RouteDelete(opts RouteDeleteOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}
	root := opts.Root
	if root == "" {
		root = "."
	}

	snakeDomain := ToSnake(opts.Domain)
	routeType := opts.Domain + "Route"
	relFile := filepath.Join(routesBasePath, snakeDomain+".go")
	absFile := filepath.Join(root, relFile)

	if _, err := os.Stat(absFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("arquivo de rotas não existe: %s", relFile)
		}
		return "", fmt.Errorf("stat %s: %w", relFile, err)
	}

	if err := assertNoDeclarableEntry(root, snakeDomain); err != nil {
		return "", err
	}
	if err := assertRouteStructEmpty(absFile, relFile, routeType); err != nil {
		return "", err
	}

	if err := os.Remove(absFile); err != nil {
		return "", fmt.Errorf("remove %s: %w", relFile, err)
	}
	return relFile, nil
}

// assertNoDeclarableEntry falha se cmd/http/routes/declarable.go ainda tem
// a chave "<snake_domain>" no map retornado por GetRoutes. Se o arquivo
// não existir, passa por vacuidade — repos em estágio inicial podem não ter
// o registro ainda, e `route delete` não deve depender da existência dele.
func assertNoDeclarableEntry(root, snakeDomain string) error {
	absFile := filepath.Join(root, declarableRelPath)
	src, err := os.ReadFile(absFile) //nolint:gosec // G304: path derives from validated identifier
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("ler %s: %w", declarableRelPath, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", declarableRelPath, err)
	}
	fn := findFreeFuncDeclOpt(file, getRoutesFunc)
	if fn == nil {
		// Sem GetRoutes, não há registro a checar.
		return nil
	}
	mapLit, err := findRoutesMapLit(fn)
	if err != nil {
		return fmt.Errorf("em %s: %w", declarableRelPath, err)
	}
	if hasRouteEntry(mapLit, snakeDomain) {
		return fmt.Errorf("em %s: entrada %q ainda registrada em GetRoutes; remova-a antes de rodar `route delete`",
			declarableRelPath, snakeDomain)
	}
	return nil
}

// assertRouteStructEmpty falha se a struct <Pascal>Route no arquivo da
// Route tem campos. Cada campo representa um handler anexado via `route add`
// e precisa ser desanexado manualmente antes do delete — NAVE-92 cobrirá
// `route remove <Domain> <Service>` simétrico; até lá, listar os campos no
// erro dá ao operador a info necessária.
func assertRouteStructEmpty(absFile, relFile, routeType string) error {
	src, err := os.ReadFile(absFile) //nolint:gosec // G304: path derives from validated identifier
	if err != nil {
		return fmt.Errorf("ler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), absFile, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", relFile, err)
	}
	st, err := findStructType(file, routeType)
	if err != nil {
		return fmt.Errorf("em %s: %w", relFile, err)
	}
	if st.Fields != nil && len(st.Fields.List) > 0 {
		names := collectFieldNames(st)
		return fmt.Errorf("em %s: struct %s tem handlers anexados [%s]; desfaça os `route add` (ou edite o arquivo) antes de rodar `route delete`",
			relFile, routeType, strings.Join(names, ", "))
	}
	return nil
}

// findStructType localiza a *ast.StructType de um type top-level pelo nome.
// Diferente de hasStruct, devolve o nó pra inspeção dos Fields.
func findStructType(file *ast.File, typeName string) (*ast.StructType, error) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || ts.Name.Name != typeName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("tipo %s não é struct", typeName)
			}
			return st, nil
		}
	}
	return nil, fmt.Errorf("struct %s não encontrada", typeName)
}

// collectFieldNames extrai os nomes (em ordem) de cada Field do FieldList.
// Campos anônimos (embedding) aparecem como o nome do tipo. Mantém estável
// pra mensagem de erro determinística entre runs.
func collectFieldNames(st *ast.StructType) []string {
	if st.Fields == nil {
		return nil
	}
	var names []string
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			if id, ok := f.Type.(*ast.Ident); ok {
				names = append(names, id.Name)
				continue
			}
			names = append(names, "_")
			continue
		}
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

// findFreeFuncDeclOpt é a variante non-erroring de findFreeFuncDecl: devolve
// nil quando a função não existe (em vez de erro). Útil quando a ausência da
// função é estado válido — não um erro de validação.
func findFreeFuncDeclOpt(file *ast.File, funcName string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name == nil {
			continue
		}
		if fd.Name.Name == funcName && fd.Body != nil {
			return fd
		}
	}
	return nil
}
