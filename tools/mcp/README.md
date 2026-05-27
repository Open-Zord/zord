# zord-mcp

Servidor [Model Context Protocol](https://modelcontextprotocol.io/) que
expõe o `tools/scaffold/` (gerador AST) e o
`tools/arch_analyser/` (validações de arquitetura) como **tools** e
**resources** consumíveis por clientes MCP (Claude Code, IDEs, etc.).

Entrypoint: [`cmd/mcp/main.go`](../../cmd/mcp/main.go). Transport: stdio
único. SDK: `github.com/modelcontextprotocol/go-sdk`.

## Rodar

O servidor recebe o path do repo alvo via `--repo` (ou env
`ZORD_REPO`); na ausência dos dois, usa o diretório de trabalho atual
(cwd). Fala JSON-RPC em stdio.

```bash
# direto (desenvolvimento)
go run ./cmd/mcp --repo .

# compilado
go build -o /tmp/mcp-bin ./cmd/mcp
/tmp/mcp-bin --repo /caminho/do/repo
```

Logs estruturados em **stderr** (stdout é reservado pro framing JSON-RPC).

Para conectar via Claude Code: o arquivo [`.mcp.json`](../../.mcp.json) na
raiz do projeto já registra `zord-mcp` apontando para
`go run ./cmd/mcp --repo .`. Em outros clientes, replicar o `command`/
`args` correspondentes.

### Override per-call do repo (worktrees)

`--repo` define apenas o **default** do servidor. Toda tool de escrita
(`scaffold_*`) e a `arch_analyze` aceitam um argumento opcional `repo`
no input que sobrescreve esse default só para a chamada — útil quando o
mesmo processo MCP atende várias worktrees em paralelo.

```jsonc
// chamada pra uma worktree, sem mexer no checkout principal
{
  "name": "OrgMembership",
  "repo": "/home/dev/worktrees/feat-exemplo"
}
```

O override é validado a cada chamada (path existe, é diretório, tem
`go.mod`). O lock cross-process (`.scaffold/lock`) segue o repo da
chamada, não o do startup — duas worktrees podem rodar scaffold em
paralelo sem disputar flock.

## Tools (29)

Todas devolvem `CommonOutput {created, modified, deleted?, diff?, warnings?}`,
exceto `arch_analyze` (output próprio). Tools destrutivas (5 `*_delete`) são
marcadas com `DestructiveHint=true` nas annotations MCP.

| Tool                            | Descrição                                                                 |
|---------------------------------|---------------------------------------------------------------------------|
| `scaffold_domain_create`        | Gera `internal/application/domain/<snake>/<snake>.go`                    |
| `scaffold_domain_delete`        | ⚠ Remove `internal/application/domain/<snake>/` (falha com deps)         |
| `scaffold_field_add`            | Adiciona campo à struct de domínio (tags db/json/validate canônicas)     |
| `scaffold_field_remove`         | Remove campo da struct                                                    |
| `scaffold_field_set_tag`        | [placeholder] substitui valor de tag — não implementado                  |
| `scaffold_field_set_type`       | [placeholder] substitui tipo de campo — não implementado                 |
| `scaffold_derive_schema`        | Deriva bloco HCL Atlas a partir do domínio Go                            |
| `scaffold_derive_schema_remove` | Remove bloco HCL Atlas do domínio (apaga conteúdo entre sentinelas)      |
| `scaffold_repository_create`    | Gera `internal/repositories/<snake>/repository.go` (sqlx)                |
| `scaffold_repository_delete`    | ⚠ Remove `internal/repositories/<snake>/` (falha se ainda registrado)    |
| `scaffold_repository_port`      | Adiciona port + métodos no arquivo do domínio                            |
| `scaffold_repository_unport`    | Remove port + métodos do arquivo do domínio (inverso de `_port`)         |
| `scaffold_repository_register`  | Registra repository no DI (`bootstrap/repository.go`)                    |
| `scaffold_repository_unregister`| Desregistra repository do DI (inverso de `_register`)                    |
| `scaffold_service_create`       | Gera `internal/application/services/<d>/<v>/service.go`                  |
| `scaffold_service_delete`       | ⚠ Remove pasta do service (falha se ainda registrado)                    |
| `scaffold_service_register`     | Registra service no DI (`bootstrap/service.go`)                          |
| `scaffold_service_unregister`   | Desregistra service do DI (inverso de `_register`)                       |
| `scaffold_handler_create`       | Gera `cmd/http/handlers/<d>/<v>/handler.go` (eager registry.Resolve)     |
| `scaffold_handler_delete`       | ⚠ Remove pasta do handler (falha se ainda registrado)                    |
| `scaffold_handler_register`     | Registra handler no DI (`bootstrap/handler.go`)                          |
| `scaffold_handler_unregister`   | Desregistra handler do DI (inverso de `_register`)                       |
| `scaffold_route_create`         | Gera `cmd/http/routes/<snake>.go` (struct + constructor)                 |
| `scaffold_route_delete`         | ⚠ Remove `cmd/http/routes/<snake>.go` (falha se ainda registrado)        |
| `scaffold_route_add`            | Adiciona método HTTP (GET/POST/...) numa Route existente                 |
| `scaffold_route_remove`         | Remove método HTTP da Route (inverso de `_add`)                          |
| `scaffold_route_register`       | Registra constructor no `GetRoutes` map (`cmd/http/routes/declarable.go`) |
| `scaffold_route_unregister`     | Desregistra constructor do map (inverso de `_register`)                  |
| `arch_analyze`                  | Roda todas as validações de arquitetura; devolve violações estruturadas  |

## Resources (4)

| URI                              | Tipo         | Conteúdo                                                |
|----------------------------------|--------------|---------------------------------------------------------|
| `scaffold://docs/readme`         | Resource     | `tools/scaffold/README.md` (text/markdown)              |
| `scaffold://docs/conventions`    | Resource     | Convenções do scaffold (IDs, AST, HCL, DI, camadas)     |
| `scaffold://domains`             | Resource     | JSON `{"domains":["auth", ...]}`                       |
| `scaffold://domain/{name}`       | Template     | Source Go do arquivo do domain (text/x-go)              |

`{name}` é validado contra `[a-z0-9_]` (anti path-traversal). Domain
inexistente devolve `Resource not found`.

## Lock cross-process

Os handlers que tocam arquivos compartilhados (bootstrap/*, declarable.go,
schema HCL, pasta do próprio domain via domain_delete que valida deps no
schemas/) pegam `flock(LOCK_EX)` em `<repo>/.scaffold/lock` antes de chamar
o scaffold:

- `scaffold_derive_schema`
- `scaffold_derive_schema_remove`
- `scaffold_domain_delete`
- `scaffold_repository_register`
- `scaffold_repository_unregister`
- `scaffold_service_register`
- `scaffold_service_unregister`
- `scaffold_handler_register`
- `scaffold_handler_unregister`
- `scaffold_route_register`
- `scaffold_route_unregister`

Garante serialização entre múltiplos clientes MCP concorrentes (ou MCP
+ invocação direta do binário scaffold). Liberação automática no close
do fd. O arquivo `.scaffold/lock` é vazio e persiste — é só um sentinel
de coordenação.

Tools que tocam só arquivos privados ao domínio (`repository_port`/
`repository_unport`, `route_add`/`route_remove`) ou apenas removem
diretório inteiro sem ler estado compartilhado (`repository_delete`,
`service_delete`, `handler_delete`, `route_delete`) não pegam lock.

## Layout interno

```
tools/mcp/
├── server.go              # newServer; capabilities mínimas
├── run.go                 # Run(ctx, args); conecta stdio transport
├── repo.go                # resolveRepo (flag --repo / env ZORD_REPO / cwd)
├── registry.go            # registerTools; annotations padrão
├── result.go              # CommonOutput; createdOutput/modifiedOutput
├── lock.go                # withRepoLock (flock)
├── tool_domain.go         # 2 tools (1 lock, 1 destructive)
├── tool_field.go          # 4 tools (2 placeholders) + parseFieldTags
├── tool_derive.go         # 1 tool (lock)
├── tool_repository.go     # 5 tools (2 lock, 1 destructive)
├── tool_service.go        # 3 tools (2 lock, 1 destructive)
├── tool_handler.go        # 3 tools (2 lock, 1 destructive)
├── tool_route.go          # 5 tools (2 lock, 1 destructive)
├── tool_arch.go           # 1 tool (arch_analyze) + lista de validadores
├── resources.go           # registerResources
├── resources_docs.go      # readme + conventions
└── resources_domains.go   # lista + template detail
```

## Adicionar uma tool

1. Criar `tool_<area>.go` (ou estender um existente) com:
   - struct `<area><Op>Input` com tags `json:` + `jsonschema:` (o SDK
     infere o schema JSON via google/jsonschema-go).
   - função `register<Area>(s *mcpsdk.Server, repo string)` chamando
     `mcpsdk.AddTool[Input, Output]`.
2. Se a tool tocar arquivo compartilhado, envolver no `withRepoLock`.
3. Listar o `register<Area>` em `registry.go`.
4. Rodar `golangci-lint run ./tools/mcp/...` e o smoke test do README.
