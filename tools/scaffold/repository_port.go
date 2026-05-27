// Package scaffold (área repository) entrega o scaffold do adapter sqlx do domínio.
// `repository port` patcha o arquivo do domínio com os métodos e a interface
// que satisfazem base_repository.BaseRepository[T]; `repository create` gera
// o arquivo do repositório concreto que embeda *base_repository.BaseRepo[T].
package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// RepositoryPortOptions parametriza RepositoryPort.
type RepositoryPortOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Domain é o nome do domínio em PascalCase (ex.: "OrgMembership").
	Domain string
	// Table sobrescreve o nome da tabela usado em Schema().
	// Vazio = snake_case(Domain) + "s".
	Table string
	// MultiTenant ativa o padrão `client` (campo + setter + prefix em Schema).
	MultiTenant bool
}

// RepositoryPort adiciona ao arquivo do domínio os métodos da constraint
// base_repository.Domain (Schema, GetFilters, SoftDelete), o setter
// SetFilters, o campo não-exportado `filters`, a interface Repository
// embedando base_repository.BaseRepository[<Domain>], e os imports
// correspondentes. Se MultiTenant, adiciona também o campo `client`,
// SetClient e o prefix em Schema().
//
// Falha imediatamente — sem aplicar nada — se qualquer um dos elementos a
// gerar já existir no arquivo. Re-rodar exige limpeza manual.
func RepositoryPort(opts RepositoryPortOptions) (string, error) {
	if !IsValidExportedIdent(opts.Domain) {
		return "", fmt.Errorf("nome de domínio inválido (esperado PascalCase exportável): %q", opts.Domain)
	}

	snake := ToSnake(opts.Domain)
	table := strings.TrimSpace(opts.Table)
	if table == "" {
		table = snake + "s"
	}

	relFile, absFile := domainPaths(opts.Root, opts.Domain)
	fset := token.NewFileSet()
	//nolint:gosec // G304: path derives from validated domain name, not raw user input
	src, err := os.ReadFile(absFile)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", relFile, err)
	}
	file, err := parser.ParseFile(fset, absFile, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", relFile, err)
	}

	st, err := findDomainStruct(file, opts.Domain)
	if err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	if err := assertNoConflicts(file, st, opts); err != nil {
		return "", fmt.Errorf("em %s: %w", relFile, err)
	}

	receiver := strings.ToLower(opts.Domain[:1])

	if opts.MultiTenant {
		appendUnexportedField(st, "client", Ident("string"))
	}
	appendUnexportedField(st, "filters", StarOf(Sel("filters", "Filters")))

	decls := buildPortDecls(receiver, opts.Domain, table, opts.MultiTenant)
	padder := NewLinePadder(fset, "scaffold-repository-port")
	padder.StampDecls(decls...)
	file.Decls = append(file.Decls, decls...)

	imp, err := newImportPaths(opts.Root)
	if err != nil {
		return "", err
	}
	astutil.AddImport(fset, file, imp.join(filtersImportSubpath))
	astutil.AddImport(fset, file, imp.join(baseRepositoryImportSubpath))

	if err := writeFile(absFile, fset, file); err != nil {
		return "", err
	}
	return relFile, nil
}

// assertNoConflicts retorna erro se qualquer elemento que RepositoryPort iria gerar já
// existe no arquivo. A checagem é feita antes de qualquer mutação para que
// uma falha não deixe o arquivo em estado intermediário.
func assertNoConflicts(file *ast.File, st *ast.StructType, opts RepositoryPortOptions) error {
	methods := []string{"Schema", "GetFilters", "SoftDelete", "SetFilters"}
	if opts.MultiTenant {
		methods = append(methods, "SetClient")
	}
	for _, m := range methods {
		if hasMethod(file, opts.Domain, m) {
			return fmt.Errorf("método %s.%s já existe", opts.Domain, m)
		}
	}

	if hasUnexportedField(st, "filters") {
		return fmt.Errorf("campo %s.filters já existe", opts.Domain)
	}
	if opts.MultiTenant && hasUnexportedField(st, "client") {
		return fmt.Errorf("campo %s.client já existe", opts.Domain)
	}

	if hasTypeDecl(file, "Repository") {
		return fmt.Errorf("tipo Repository já existe em %s", file.Name.Name)
	}
	return nil
}

func appendUnexportedField(st *ast.StructType, fieldName string, typ ast.Expr) {
	st.Fields.List = append(st.Fields.List, Field(fieldName, typ))
}

// buildPortDecls constrói, via AST puro, os Decls a anexar ao arquivo do
// domínio: setters (pointer receivers), métodos da constraint (value
// receivers) e a interface Repository. Ordem mimica usage_record.go:
// SetClient → SetFilters → SoftDelete → GetFilters → Schema → interface.
func buildPortDecls(receiver, domain, table string, multiTenant bool) []ast.Decl {
	stringResult := FieldList(AnonField(Ident("string")))
	filtersType := Sel("filters", "Filters")

	var decls []ast.Decl

	if multiTenant {
		// func (r *Domain) SetClient(client string) { r.client = client }
		decls = append(decls, FuncDecl(
			PointerReceiver(receiver, domain),
			"SetClient",
			FieldList(Field("client", Ident("string"))),
			nil,
			[]ast.Stmt{Assign(Sel(receiver, "client"), Ident("client"))},
		))
	}

	// func (r *Domain) SetFilters(f *filters.Filters) { r.filters = f }
	decls = append(decls, FuncDecl(
		PointerReceiver(receiver, domain),
		"SetFilters",
		FieldList(Field("f", StarOf(filtersType))),
		nil,
		[]ast.Stmt{Assign(Sel(receiver, "filters"), Ident("f"))},
	))

	// func (r Domain) SoftDelete() string { return "deleted_at" }
	decls = append(decls, FuncDecl(
		ValueReceiver(receiver, domain),
		"SoftDelete",
		nil,
		stringResult,
		[]ast.Stmt{ReturnStmt(StrLit("deleted_at"))},
	))

	// func (r Domain) GetFilters() filters.Filters {
	//     if r.filters != nil { return *r.filters }
	//     return filters.Filters{}
	// }
	decls = append(decls, FuncDecl(
		ValueReceiver(receiver, domain),
		"GetFilters",
		nil,
		FieldList(AnonField(Sel("filters", "Filters"))),
		[]ast.Stmt{
			IfStmt(
				Binary(token.NEQ, Sel(receiver, "filters"), Ident("nil")),
				[]ast.Stmt{ReturnStmt(StarOf(Sel(receiver, "filters")))},
			),
			ReturnStmt(CompositeLit(Sel("filters", "Filters"))),
		},
	))

	// func (r Domain) Schema() string { ... }
	var schemaBody []ast.Stmt
	if multiTenant {
		schemaBody = []ast.Stmt{
			IfStmt(
				Binary(token.EQL, Sel(receiver, "client"), StrLit("")),
				[]ast.Stmt{ReturnStmt(StrLit(table))},
			),
			ReturnStmt(Binary(token.ADD,
				Binary(token.ADD, Sel(receiver, "client"), StrLit(".")),
				StrLit(table),
			)),
		}
	} else {
		schemaBody = []ast.Stmt{ReturnStmt(StrLit(table))}
	}
	decls = append(decls, FuncDecl(
		ValueReceiver(receiver, domain),
		"Schema",
		nil,
		stringResult,
		schemaBody,
	))

	// type Repository interface { base_repository.BaseRepository[Domain] }
	decls = append(decls, TypeDecl("Repository", &ast.InterfaceType{
		Methods: FieldList(AnonField(
			IndexExpr(Sel("base_repository", "BaseRepository"), Ident(domain)),
		)),
	}))

	return decls
}

func hasUnexportedField(st *ast.StructType, fieldName string) bool {
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == fieldName {
				return true
			}
		}
	}
	return false
}

// hasMethod retorna true se já existe um FuncDecl com receiver no tipo
// (ponteiro ou valor) e o nome de método informado.
func hasMethod(file *ast.File, recvType, method string) bool {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || fd.Name == nil || fd.Name.Name != method {
			continue
		}
		if receiverMatches(fd.Recv, recvType) {
			return true
		}
	}
	return false
}

func receiverMatches(recv *ast.FieldList, recvType string) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	id, ok := t.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == recvType
}

// hasTypeDecl retorna true se file já declara um TypeSpec com o nome dado.
func hasTypeDecl(file *ast.File, typeName string) bool {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name.Name == typeName {
				return true
			}
		}
	}
	return false
}
