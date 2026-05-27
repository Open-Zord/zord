package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
)

const sqlxImportPath = "github.com/jmoiron/sqlx"

// RepositoryCreateOptions parametriza RepositoryCreate.
type RepositoryCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "OrgMembership").
	Domain string
}

// RepositoryCreate gera internal/repositories/<snake>/<snake>.go com o pacote
// `<snake>_repository`, uma struct concreta embedando
// *base_repository.BaseRepo[<snake>.<Domain>] e o constructor
// New<Domain>Repository(mysql *sqlx.DB). Retorna o caminho relativo à raiz.
//
// Falha se o arquivo já existe, se o arquivo do domínio não existe ou se a
// struct do domínio não existe nesse arquivo (o repo concreto importa o tipo
// e ficaria inválido).
func RepositoryCreate(opts RepositoryCreateOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}

	if err := assertDomainStructExists(opts.Root, opts.Domain); err != nil {
		return "", err
	}

	snake := ToSnake(opts.Domain)
	root := opts.Root
	if root == "" {
		root = "."
	}
	relDir := filepath.Join(repositoriesBasePath, snake)
	relFile := filepath.Join(relDir, snake+".go")
	absDir := filepath.Join(root, relDir)
	absFile := filepath.Join(root, relFile)

	if _, err := os.Stat(absFile); err == nil {
		return "", fmt.Errorf("repository já existe: %s", relFile)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", absFile, err)
	}

	imp, err := newImportPaths(root)
	if err != nil {
		return "", err
	}
	src, err := buildConcreteFile(opts.Domain, imp)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", absDir, err)
	}
	if err := os.WriteFile(absFile, src, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", absFile, err)
	}
	return relFile, nil
}

// buildConcreteFile monta, via AST puro, o arquivo Go do repository concreto:
//
//	package <snake>_repository
//
//	import (
//	    "github.com/jmoiron/sqlx"
//
//	    "<module>/internal/application/domain/<snake>"
//	    "<module>/internal/repositories/base_repository"
//	)
//
//	// RegistryKey identifica o *<Domain>Repository no pkg/registry.
//	const RegistryKey = "<lowerCamel>Repository"
//
//	type <Domain>Repository struct {
//	    *base_repository.BaseRepo[<snake>.<Domain>]
//	}
//
//	func New<Domain>Repository(mysql *sqlx.DB) *<Domain>Repository {
//	    return &<Domain>Repository{
//	        BaseRepo: base_repository.NewBaseRepository[<snake>.<Domain>](mysql),
//	    }
//	}
//
// O const é obrigatório pra `scaffold repository register` conseguir
// referenciar `<alias>.RegistryKey` no map central em
// `bootstrap/repositories.go` (NAVE-106).
func buildConcreteFile(domain string, imp importPaths) ([]byte, error) {
	snake := ToSnake(domain)
	pkg := snake + "_repository"
	repoType := domain + "Repository"

	// Declaração do const RegistryKey com valor `<lowerCamel>Repository`.
	registryKey := ToLowerCamel(domain) + "Repository"
	constDecl := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{Ident("RegistryKey")},
				Values: []ast.Expr{StrLit(registryKey)},
			},
		},
	}
	constDecl.Doc = singleComment(fmt.Sprintf("// RegistryKey identifica o *%s no pkg/registry.", repoType))

	// *base_repository.BaseRepo[<snake>.<Domain>]
	baseRepoIndexed := IndexExpr(Sel("base_repository", "BaseRepo"), Sel(snake, domain))
	baseRepoPointer := StarOf(baseRepoIndexed)

	structDecl := TypeDecl(repoType, &ast.StructType{
		Fields: FieldList(AnonField(baseRepoPointer)),
	})

	// base_repository.NewBaseRepository[<snake>.<Domain>](mysql)
	newBaseCall := &ast.CallExpr{
		Fun:  IndexExpr(Sel("base_repository", "NewBaseRepository"), Sel(snake, domain)),
		Args: []ast.Expr{Ident("mysql")},
	}

	// &<Domain>Repository{ BaseRepo: <newBaseCall> } — multi-line forçado abaixo.
	compLit := &ast.CompositeLit{
		Type: Ident(repoType),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: Ident("BaseRepo"), Value: newBaseCall},
		},
	}
	returnLit := &ast.UnaryExpr{Op: token.AND, X: compLit}

	constructor := FuncDecl(
		nil,
		"New"+repoType,
		FieldList(Field("mysql", StarOf(Sel("sqlx", "DB")))),
		FieldList(AnonField(StarOf(Ident(repoType)))),
		[]ast.Stmt{ReturnStmt(returnLit)},
	)

	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-repository-create")

	// Único import group com dois sub-grupos (stdlib/third-party + locais)
	// separados por blank line interna.
	imports := ImportGroups(padder,
		[]string{sqlxImportPath},
		[]string{
			imp.join(domainImportSubpath + "/" + snake),
			imp.join(baseRepositoryImportSubpath),
		},
	)
	// gap entre imports e a próxima Decl é tratado por StampDecls abaixo.

	file := &ast.File{
		Name:  Ident(pkg),
		Decls: []ast.Decl{imports, constDecl, structDecl, constructor},
	}
	padder.StampDecls(constDecl, structDecl, constructor)
	StampCompositeLit(padder, compLit)

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("formatar AST: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w\n%s", err, buf.String())
	}
	if !bytes.HasSuffix(formatted, []byte("\n")) {
		formatted = append(formatted, '\n')
	}
	return formatted, nil
}
