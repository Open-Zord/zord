# scaffold

Gerador de código backend, construído camada a camada com AST puro.

**Status:** em produção. Gerador único de código backend.

## Princípios

1. **AST puro.** `go/ast` + `go/format` para Go, `hclwrite` para HCL. Sem `text/template`, sem `jennifer`, sem regex em patches.
2. **Domínio é fonte de verdade.** Todo artefato Go de dados (domain, DTO, request, response) é editado via comandos granulares. Representações não-Go (HCL Atlas etc.) são derivadas via comando explícito.
3. **Um comando = um artefato.** Sem orquestrador agregador. Composição entre camadas é responsabilidade do spec da task, não do gerador.
4. **Idempotência por falha clara.** Comandos de edição falham em duplicado; para mutação existem comandos explícitos (`set-tag`, `set-type`).

Proposta de design: novo gerador de código backend (AST puro).

## Comandos disponíveis

Fatia 1 (NAVE-56 + NAVE-95):

```
scaffold domain create <Name>
scaffold domain delete <Domain> [--table <name>] [--schema-path <path>]
scaffold field add <Domain> <Field> <Type> [--tag-*]
scaffold field remove <Domain> <Field>
```

Fatia 2 (NAVE-57 + NAVE-94) — primeira camada derivada:

```
scaffold derive schema <Domain> [--table <name>] [--schema-path <path>] [--schema-name <name>]
scaffold derive schema <Domain> --remove [--table <name>] [--schema-path <path>]
```

Fatia 3 (NAVE-58) — adapter sqlx:

```
scaffold repository port <Domain> [--multi-tenant] [--table <name>]
scaffold repository unport <Domain>
scaffold repository create <Domain>
scaffold repository delete <Domain>
```

Fatia 4 (NAVE-59 + NAVE-124) — esqueleto de service no padrão zord
(trio request.go / service.go / response.go):

```
scaffold service create <Domain> <Verb>

scaffold request field add <Domain> <Verb> <Field> <Type> [--validate=...]
scaffold request field remove <Domain> <Verb> <Field>
scaffold request validator set <Domain> <Verb>
scaffold request validator unset <Domain> <Verb>

scaffold response field add <Domain> <Verb> <Field> <Type>
scaffold response field remove <Domain> <Verb> <Field>
```

O `service create` agora gera três arquivos por verbo dentro de
`internal/application/services/<snake_domain>/<snake_verb>/`:

  * `request.go` — `Data` (entrada validável), `Request` (encapsula `Data`),
    `NewRequest` e `Validate` (no-op até `request validator set` ser rodado).
  * `service.go` — `RegistryKey`, `Service` embedando `services.BaseService`,
    `NewService(logger, idCreator)`, `Execute(ctx, *Request)` e
    `GetResponse() (*Response, *services.Error)`.
  * `response.go` — `Response` (saída do use case).

Os comandos `request field`/`response field` operam sobre `Data` e
`Response` via AST e sempre adicionam tag `json:"<snake>"`. O
`request validator set` regenera `request.go` na variante com validador,
injetando o parâmetro `validator services.Validator` em `NewRequest` e
trocando o body de `Validate` por um loop sobre
`r.validator.ValidateStruct(r.Data)`. O `unset` volta à variante sem
validador.

Fatia 5 (NAVE-70, substitui NAVE-61; refatorada em NAVE-77 para eager
deps no constructor; adaptada em NAVE-124 ao novo padrão zord do service) —
adapter HTTP:

```
scaffold handler create <Domain> <Service>
```

O handler gerado faz `c.Bind(&data)` no tipo `<service>.Data`, chama
`<service>.NewRequest(&data)` (ou `NewRequest(&data, h.validator)` quando
o request tem validator) e usa `h.svc.Execute(ctx, req)` +
`h.svc.GetResponse()` no padrão novo. A detecção de validator vem da
inspeção AST do `request.go` correspondente.

Fatia 6 (NAVE-63 + NAVE-74) — registro de rotas por domain, com Route
recebendo só `*registry.Registry` no constructor e resolvendo handlers
internamente (eager):

```
scaffold route create <Domain>
scaffold route add <Domain> <Service> --method=<M> [--path=<p>] [--public]
```

Fatia 8 (NAVE-60) — wire-up de service no DI:

```
scaffold service register <Domain> <Verb>
```

Fatia 9 (NAVE-72 + NAVE-89 + NAVE-96) — wire-up, wire-down e delete de repository:

```
scaffold repository register <Domain>
scaffold repository unregister <Domain>
scaffold repository delete <Domain>
```

Fatia 10 (NAVE-65) — registro da Route no map central:

```
scaffold route register <Domain>
```

Cleanup simétrico de `route create` (NAVE-99):

```
scaffold route delete <Domain>
```

Fatia 11 (NAVE-73 + NAVE-98) — wire-up, wire-down e remoção de handler:

```
scaffold handler register <Domain> <Service>
scaffold handler unregister <Domain> <Service>
scaffold handler delete <Domain> <Service>
```

Fatia 12 (NAVE-87) — projection no domain (struct agregada para queries
custom com GROUP BY):

```
scaffold projection create <Domain> <ProjectionName>
scaffold projection field add <Domain> <ProjectionName> <Field> <Type> [--tag-db=<col>] [--no-db-tag]
scaffold projection field remove <Domain> <ProjectionName> <Field>
```

Fatias seguintes ainda não implementadas: test e2e (NAVE-64 cancelada).

## Fluxo end-to-end de um use case novo (pós-NAVE-124)

```
scaffold domain create Foo
scaffold field add Foo Bar string --tag-db=bar --tag-db-pk
scaffold derive schema Foo
scaffold repository port Foo
scaffold repository create Foo
scaffold service create Foo Create
scaffold request field add Foo Create Email string --validate=required,email
scaffold response field add Foo Create ID string
scaffold request validator set Foo Create     # opcional — só se houver validate tags
scaffold handler create Foo Create
scaffold service register Foo Create
scaffold handler register Foo Create
scaffold route create Foo                      # primeira vez no domínio
scaffold route add Foo Create --method=POST --path=/foos
scaffold route register Foo                    # primeira vez no domínio
```

O handler detecta automaticamente se o `request.go` está na variante com
validator (assinatura de `NewRequest` com 2 parâmetros) e injeta o
`*services.Validator` resolvido do registry. Sem validator configurado o
handler usa a assinatura de 1 parâmetro.

## `derive schema` — regeneração do bloco HCL

`scaffold derive schema <Domain>` lê o domínio Go em `internal/application/domain/<snake>/<snake>.go` e regenera o bloco `table` correspondente em `schemas/schema.my.hcl`. O bloco fica envolvido por sentinelas:

```
# scaffold:generated <table>
table "<table>" { ... }
# scaffold:end <table>
```

Reexecuções substituem o bloco entre sentinelas; conteúdo fora delas é preservado byte-a-byte. O nome da tabela default é `snake_case(Domain) + "s"` (override via `--table`).

### Mapeamento tag → HCL

| Tag Go | Resultado HCL |
|---|---|
| `db:"col"` | `column "col"` |
| sem `db_type`, `string` | `type = varchar(255)` ou `varchar(<db_size>)` |
| sem `db_type`, `int64` | `type = bigint` |
| sem `db_type`, `int32` / `int` | `type = int` |
| sem `db_type`, `bool` | `type = tinyint(1)` |
| sem `db_type`, `time.Time` | `type = datetime` |
| `db_type:"X"` sem `db_size` | `type = X` (identifier) |
| `db_type:"X" db_size:"N"` | `type = X(N)` — `db_size` aceita lista CSV (ex.: `"16,6"` → `X(16, 6)`) |
| `*T` (pointer) | `null = true` |
| `T` (valor) | `null = false` |
| `db_pk:""` (presença) | inclui em `primary_key { columns = [...] }` |
| `db_fk:"tabela.coluna"` | gera `foreign_key "fk_<table>_<col>" { ...; on_delete = CASCADE }` |
| `db_index:"true"` | gera `index "idx_<table>_<col>" { columns = [...] }` |
| `db_index:"unique"` | gera `index "idx_<table>_<col>_uq" { ...; unique = true }` |

### Contrato da sentinela

- **Idempotência:** mesma derivação não altera bytes do arquivo.
- **Bloco gerado é exclusivo do scaffold.** Edições manuais dentro das sentinelas são perdidas na próxima derivação.
- **Sentinela parcial** (`:generated` sem `:end` ou vice-versa) → erro.
- **`table "<name>"` pré-existente sem sentinela** → erro. Para "adotar" uma tabela manual, envolver no par de sentinelas e rodar novamente.

### Remoção (`--remove`)

`scaffold derive schema <Domain> --remove` apaga o bloco delimitado pelas
sentinelas `# scaffold:generated <table>` / `# scaffold:end <table>` em
`schemas/schema.my.hcl`, preservando o restante do arquivo byte-a-byte.
Inverso simétrico de `derive schema` — fecha o ciclo no fluxo de
desmontagem de domínio (`domain delete` recusa apagar enquanto o bloco
HCL ainda existe).

- **Não lê o pacote Go do domínio.** Opera apenas sobre o nome da
  tabela (default `snake_case(Domain) + "s"`, override via `--table`).
  Assim funciona mesmo após o `.go` do domínio já ter sido removido.
- **Não aceita `--schema-name`** — irrelevante para remoção.
- **Idempotência inversa:** re-rodar após sucesso falha com
  `sentinela ausente para table "<table>"`.
- **Sentinela parcial** (`:generated` sem `:end`) → erro, arquivo
  intacto.
- **Linha em branco residual** entre vizinhos é aceitável (mesmo
  trade-off do `derive` no append).

Exemplo:

```bash
go run ./cmd/scaffold derive schema Widget --remove
# removido de: schemas/schema.my.hcl
go run ./cmd/scaffold derive schema Widget --remove
# scaffold: unpatch schemas/schema.my.hcl: sentinela ausente para table "widgets"
```

## `service register` — wire-up no DI

`scaffold service register <Domain> <Verb>` patcha `bootstrap/services.go` via AST adicionando:

1. **Import** do pacote do verbo: `"zord/internal/application/services/<snake_domain>/<snake_verb>"`.
   - Bare quando o nome do pacote (`<snake_verb>`) é único entre os imports do arquivo.
   - Alias `<snake_domain>_<snake_verb>` quando há colisão (ex.: `Org.Create` já registrado e novo `Billing.Create` chega).
2. **Linha de registro** ao fim de `registerServices`:
   ```go
   reg.Provide(<pkg>.RegistryKey, <pkg>.NewService(log, idC))
   ```
   Apenas as dependências universais (`log`, `idC`) — exatamente o que `service create` gera. Quando o dev evolui o constructor (adiciona ports), a chamada quebra compilação até ser atualizada manualmente. É o sinal: scaffold registra, dev cabeia.

### Idempotência

- Falha se o import OU a linha de `Provide` já existirem.
- Sem mutação parcial: se qualquer validação falha, `bootstrap/services.go` permanece byte-a-byte igual ao estado anterior.

### Pré-condições

- O service existe (`internal/application/services/<snake_domain>/<snake_verb>/service.go` com `const RegistryKey` + `func NewService`). Rode antes: `scaffold service create <Domain> <Verb>`.
- `bootstrap/services.go` existe e contém a função `registerServices(reg *registry.Registry)`.

### Limitações conhecidas (fatia 2)

- Sem suporte a `default = sql(...)` — sentinela apagaria.
- Sem índices multi-coluna.
- Sem tipos compostos (slice, map, struct embedded) no domínio.
- Migração Atlas em si (`atlas migrate diff`) continua manual.

## `service unregister` — desfaz a ligação no DI

`scaffold service unregister <Domain> <Verb>` edita `bootstrap/services.go` via AST removendo o que `service register` adicionou:

1. **Import** do pacote do verbo (`"zord/internal/application/services/<snake_domain>/<snake_verb>"`).
   - Detecta o formato real do import. Bare (sem alias) ou aliased (`<snake_domain>_<snake_verb>`) — ambos suportados.
2. **Linha de registro** correspondente em `registerServices`:
   ```go
   reg.Provide(<pkg>.RegistryKey, _)
   ```
   O segundo argumento é ignorado: na prática o dev evolui `NewService(log, idC)` adicionando ports do domínio, e o unregister precisa funcionar após essa evolução. A chave única é `<pkg>.RegistryKey`.

### Idempotência

- Falha se o import OU a linha de `Provide` não existirem.
- Sem mutação parcial: se qualquer validação falha, `bootstrap/services.go` permanece byte-a-byte igual ao estado anterior.
- Re-executar após sucesso anterior sempre falha (import e linha de `Provide` já foram removidos).

### O que NÃO faz

- **Não apaga o pacote do service.** `internal/application/services/<snake_domain>/<snake_verb>/` continua no disco — use `scaffold service delete` (abaixo) pra apagar com guardas.
- **Não inspeciona uses downstream** do `RegistryKey` em `bootstrap/handlers.go` ou em `cmd/http/routes/declarable.go`. O fluxo natural de desmontagem é:
  ```
  scaffold service unregister <Domain> <Verb>
  scaffold service delete <Domain> <Verb>
  scaffold handler unregister <Domain> <Verb>    # quando existir
  scaffold route unregister <Domain> <Verb>      # quando existir
  ```
  Quando o pacote do verbo é apagado, qualquer arquivo que ainda importe `<pkg>.RegistryKey` quebra `go build` no próximo ciclo, indicando o próximo passo. Aviso: rodar **apenas** `service unregister` sem seguir a sequência deixa o `Resolve[T](reg, <pkg>.RegistryKey)` no handler register válido em compile time, mas com `panic` em runtime.

### Pré-condições

- `bootstrap/services.go` existe e contém a função `registerServices(reg *registry.Registry)`.

## `service delete` — apaga o pacote do verbo

`scaffold service delete <Domain> <Verb>` apaga `internal/application/services/<snake_domain>/<snake_verb>/` via `os.RemoveAll`, fechando o ciclo iniciado por `service unregister`.

### Guardas

Valida em ordem (falha sem mutar disco na primeira falha):

1. **Domain e Verb** são PascalCase exportáveis.
2. **Pasta existe** (`os.Stat`). Mensagem: `service <relDir> não existe`.
3. **Wire-up ausente em `bootstrap/services.go`**:
   - Bootstrap **ausente** → segue (paridade com `service create`: o repo pode não ter bootstrap ainda).
   - Bootstrap **presente sem `registerServices`** → segue (não há wire-up possível).
   - **Import** do verbo presente → falha orientando a rodar `scaffold service unregister` antes.
   - **Provide** do verbo presente (bare ou alias `<snake_domain>_<snake_verb>`) → mesma mensagem.
4. **Handler 1:1 ausente** (`os.Stat` em `cmd/http/handlers/<snake_domain>/<snake_verb>/`). Existindo a pasta → falha orientando a apagar o handler antes.

A ordem das guardas é proposital: pasta → wire-up → handler. Pasta primeiro porque é a invariante mais barata e dá a mensagem mais útil ("nada pra apagar"). Wire-up antes do handler porque é o acoplamento mais perigoso (`Resolve` em runtime panica enquanto handler órfão só quebra build).

### Idempotência

- Re-executar após sucesso anterior sempre falha com `service <relDir> não existe`.
- Falha em qualquer guarda não muta disco.

### O que NÃO faz

- **Não verifica uses transitivos** do `RegistryKey` em `cmd/http/routes/declarable.go`. A guarda de handler 1:1 cobre o cenário real; `route unregister` cuida da camada de cima.
- **Não roda `service unregister` implicitamente.** Quebra do invariante "uma operação, uma responsabilidade" — mensagem de guarda orienta o próximo passo.

## `repository register` — wire-up no DI

`scaffold repository register <Domain>` patcha `bootstrap/repositories.go` via AST adicionando:

1. **Import** do pacote do repositório com alias fixo `<snake_domain sem underscores>repo`:
   ```go
   organizationrepo "zord/internal/repositories/organization"
   orgmembershiprepo "zord/internal/repositories/org_membership"
   ```
   Sem heurística de colisão — o sufixo `repo` é uniforme e pacotes em `internal/repositories/` jamais se chamam `<snake>repo`.
2. **Linha de registro** ao fim de `registerRepositories`:
   ```go
   reg.Provide(<alias>.RegistryKey, <alias>.New<Domain>Repository(db))
   ```
   Apenas a dependência universal `db` (`*sqlx.DB`) — exatamente o que `repository create` emite no constructor. Quando o dev evolui o repository com deps custom, a chamada quebra compilação até ser atualizada manualmente. É o sinal: scaffold registra, dev cabeia.

### Idempotência

- Falha se o import OU a linha de `Provide` já existirem.
- Sem mutação parcial: se qualquer validação falha, `bootstrap/repositories.go` permanece byte-a-byte igual ao estado anterior.

### Pré-condições

- O repositório existe (`internal/repositories/<snake_domain>/<snake_domain>.go` com `const RegistryKey` + `func New<Domain>Repository`). Rode antes: `scaffold repository create <Domain>` e adicione `RegistryKey` manualmente (o constructor não emite a constante automaticamente).
- `bootstrap/repositories.go` existe e contém a função `registerRepositories(reg *registry.Registry)`.

## `repository unregister` — desfaz a ligação no DI

`scaffold repository unregister <Domain>` edita `bootstrap/repositories.go` via AST removendo o que `repository register` adicionou:

1. **Import** com alias `<snake_domain sem underscores>repo` apontando pra `zord/internal/repositories/<snake_domain>`.
2. **Linha** `reg.Provide(<alias>.RegistryKey, _)` em `registerRepositories`.

O segundo argumento de `reg.Provide` é ignorado na busca — funciona mesmo após o dev evoluir o constructor (`NewXxxRepository(db, logger, ...)`).

### Idempotência

- Falha se o import OU a linha de `Provide` não existirem.
- Sem mutação parcial: se qualquer validação falha, `bootstrap/repositories.go` permanece byte-a-byte igual ao estado anterior.
- Re-executar após sucesso anterior sempre falha (import e linha de `Provide` já foram removidos).

### O que NÃO faz

- **Não apaga o pacote** do repositório em `internal/repositories/<snake_domain>/` — só desconecta do DI. Apagar é responsabilidade do dev (`rm -rf`).
- **Não inspeciona uses downstream** do `RegistryKey` (`bootstrap/services.go` etc.). Fluxo natural:

  ```
  scaffold repository unregister <Domain>
  rm -rf internal/repositories/<snake_domain>/   # se for descartar de vez
  # ajustar services que dependiam do repository — compile error guia
  ```

  Rodar **apenas** `repository unregister` sem ajustar os consumers deixa `Resolve[T](reg, <alias>.RegistryKey)` válido em compile time, mas com `panic` em runtime.

### Pré-condições

- `bootstrap/repositories.go` existe e contém a função `registerRepositories(reg *registry.Registry)`.
- **Não exige** o pacote do repositório existir no disco — o dev pode ter apagado antes.

## `repository delete` — apaga a pasta do repository

`scaffold repository delete <Domain>` remove `internal/repositories/<snake_domain>/` inteira via `os.RemoveAll`. Simétrico a `repository create` (NAVE-58).

### Pré-condições

- A pasta `internal/repositories/<snake_domain>/` existe.
- Não há wire-up residual em `bootstrap/repositories.go`. Quando o arquivo de bootstrap existe e contém `registerRepositories`, o comando procura via AST o `ImportSpec` do pacote (`zord/internal/repositories/<snake_domain>`) e a chamada `reg.Provide(<alias>.RegistryKey, ...)`. Se ambos presentes, falha com mensagem apontando `scaffold repository unregister <Domain>`. Usa o alias REAL do `ImportSpec` — robusto contra aliases encurtados à mão (mesma estratégia do `repository unregister`, NAVE-89).

Bootstrap ausente, função `registerRepositories` ausente, import ausente OU `Provide` ausente — todos significam "sem wire-up residual", e o delete prossegue. Não há pra onde apontar; o estado já é consistente.

### Fluxo natural de desmontagem

```
scaffold repository unregister <Domain>
scaffold repository delete <Domain>
# ajustar services que dependiam do repository — compile error guia
```

Rodar **apenas** `repository delete` com wire-up residual falha e preserva a pasta no disco (sem mutação parcial).

### Não inspeciona services downstream

O comando não varre `bootstrap/services.go` nem o resto do tree procurando uses do `RegistryKey`. O compile error após o delete é o guia. Mesma postura de `service unregister` (NAVE-88) e `repository unregister` (NAVE-89).

### Sem `--force`

Decisão upfront: nada de bypass. Se quiser apagar com wire-up residual, use `rm -rf` à mão — o scaffold mantém o invariante.

## `handler register` — wire-up no DI

`scaffold handler register <Domain> <Service>` patcha `bootstrap/handlers.go` via AST adicionando:

1. **Import** do pacote do handler com alias uniforme `<snake_domain sem _><snake_service sem _>handler`:
   ```go
   authloginhandler "zord/cmd/http/handlers/auth/login"
   usagerecordexporthandler "zord/cmd/http/handlers/usage_record/export"
   ```
   Sem heurística de colisão — o par `(snake_domain, snake_service)` é único por design da NAVE-70 (1:1 handler-por-service), portanto o alias derivado é único por construção. Coexiste sem conflito com os aliases legados `<snake_domain>handler` (`authhandler`, `orghandler`).
2. **Linha de registro** ao fim de `registerHandlers`:
   ```go
   reg.Provide(<alias>.RegistryKey, <alias>.New<Service>Handler(reg))
   ```
   Apenas a dependência universal `reg` (`*registry.Registry`) — exatamente o que `handler create` emite no constructor. Quando o dev evolui o handler com deps custom, a chamada quebra compilação até ser atualizada manualmente. É o sinal: scaffold registra, dev cabeia.

### Idempotência

- Falha se o import OU a linha de `Provide` já existirem.
- Sem mutação parcial: se qualquer validação falha, `bootstrap/handlers.go` permanece byte-a-byte igual ao estado anterior.

### Pré-condições

- O handler existe (`cmd/http/handlers/<snake_domain>/<snake_service>/handler.go` com `const RegistryKey` + `func New<Service>Handler`). Rode antes: `scaffold handler create <Domain> <Service>`.
- `bootstrap/handlers.go` existe e contém a função `registerHandlers(reg *registry.Registry)`.

## `handler unregister` — desfaz a ligação no DI

`scaffold handler unregister <Domain> <Service>` edita `bootstrap/handlers.go` via AST removendo o que `handler register` adicionou:

1. **Import** do pacote do handler (`"zord/cmd/http/handlers/<snake_domain>/<snake_service>"`) com alias uniforme `<snake_domain sem _><snake_service sem _>handler`.
   - Sem detecção dual de formato: NAVE-73 estabelece o alias por construção, então o lookup é direto. Se o import existir com outro alias (handler legado fora do shape 1:1), o comando falha por ausência — comportamento correto, handlers legados não são alvo.
2. **Linha de registro** correspondente em `registerHandlers`:
   ```go
   reg.Provide(<alias>.RegistryKey, _)
   ```
   O segundo argumento é ignorado: o dev pode ter evoluído `New<Service>Handler(reg)` adicionando deps custom, e o unregister precisa funcionar após essa evolução. A chave única é `<alias>.RegistryKey`.

### Idempotência

- Falha se o import OU a linha de `Provide` não existirem.
- Sem mutação parcial: se qualquer validação falha, `bootstrap/handlers.go` permanece byte-a-byte igual ao estado anterior.
- Re-executar após sucesso anterior sempre falha (import e linha de `Provide` já foram removidos).

### O que NÃO faz

- **Não apaga o pacote do handler.** `cmd/http/handlers/<snake_domain>/<snake_service>/` continua no disco — use `scaffold handler delete` para isso.
- **Não inspeciona uses downstream** do `RegistryKey` em `cmd/http/routes/<snake_domain>.go`. O fluxo natural de desmontagem é:
  ```
  # edite cmd/http/routes/<snake_domain>.go removendo campo/import/uso do handler
  scaffold handler unregister <Domain> <Service>
  scaffold handler delete <Domain> <Service>
  scaffold service unregister <Domain> <Service>
  ```
  Aviso: rodar **apenas** `handler unregister` sem seguir a sequência deixa o `Resolve[T](reg, <alias>.RegistryKey)` na route table válido em compile time, mas com `panic` em runtime.

### Pré-condições

- `bootstrap/handlers.go` existe e contém a função `registerHandlers(reg *registry.Registry)`.

## `handler delete` — apaga a pasta do handler

`scaffold handler delete <Domain> <Service>` apaga `cmd/http/handlers/<snake_domain>/<snake_service>/` via `os.RemoveAll` após validar, em ordem (falha sem mutar disco na primeira validação que falhar):

1. `Domain` e `Service` são PascalCase exportáveis.
2. A pasta do handler existe.
3. Sem wire-up residual em `bootstrap/handlers.go`: nem `import` do pacote do handler, nem linha `reg.Provide(<alias>.RegistryKey, ...)` em `registerHandlers`. Bootstrap ausente conta como OK; bootstrap presente sem `registerHandlers` também conta como OK.
4. Sem rota residual em `cmd/http/routes/<snake_domain>.go`: nem campo `<lowerCamel>Handler` na struct da Route, nem `import` do pacote do handler, nem chamada `r.<lowerCamel>Handler.Handle` em `DeclarePrivateRoutes` / `DeclarePublicRoutes`. Route file ausente conta como OK.

Ordem das guardas: pasta → wire-up → rota. Pasta primeiro porque é a invariante mais barata e dá a mensagem mais útil ("nada pra apagar"). Wire-up antes da rota porque `Resolve` em runtime panica, enquanto rota residual quebra build.

### Idempotência

- Re-executar após sucesso retorna erro `"handler ... não existe"` — paridade com `service delete`.
- Sem flag `--force` por design: desmontagem destrutiva exige consciência do dev sobre cada passo do ciclo inverso.

### O que NÃO faz

- **Não toca o diretório-pai `<snake_domain>`** mesmo quando vazio — paridade com `service delete`.
- **Não edita o arquivo da rota** mesmo quando a rota residual é detectada — só reporta. `scaffold route remove` ainda não existe (NAVE-92, backlog), então a mensagem aponta edição manual.
- **Não detecta handlers legados com aliases hand-editados** — alias canônico (NAVE-70) é único por construção. Handlers pré-NAVE-70 ficam fora do alvo, igual `handler unregister`.

### Pré-condições

- A pasta `cmd/http/handlers/<snake_domain>/<snake_service>/` existe.

## `domain delete` — apaga a pasta do domínio com guarda de dependências

`scaffold domain delete <Domain>` remove `internal/application/domain/<snake>/` (a pasta inteira), mas só quando nenhuma camada downstream ainda referencia o domínio. Inverso simétrico de `scaffold domain create` (NAVE-56).

### Detecção de dependências residuais

Antes de mutar disco, o comando confere:

1. `internal/application/services/<snake>/...` (qualquer verbo)
2. `internal/repositories/<snake>/...`
3. `cmd/http/handlers/<snake>/...` (qualquer service)
4. `cmd/http/routes/<snake>.go`
5. Bloco `# scaffold:generated <table>` ... `# scaffold:end <table>` em `schemas/schema.my.hcl` (default `<snake>s`, sobrescrevível por `--table`).

Se qualquer uma existir, o comando acumula **todas** num único erro listando os caminhos encontrados + os comandos que limpam cada camada (`service delete`, `repository delete`, `handler delete`, `route delete`, `derive schema --remove`). **Nada é removido até o ambiente estar limpo.**

### Idempotência

- Falha se a pasta `internal/application/domain/<snake>/` não existir (inverso simétrico de `domain create`, que falha em duplicata).
- Falha se qualquer dep residual existir — sem mutação parcial.
- Re-executar após sucesso anterior sempre falha (pasta já foi removida).

### Flags

- `--root <path>` — raiz do repositório (default: cwd).
- `--table <name>` — nome da tabela checada no HCL (default: `<snake>s`).
- `--schema-path <path>` — caminho do arquivo HCL (default: `schemas/schema.my.hcl`).

### O que NÃO faz

- **Não faz cascade.** Não chama `service delete` / `repository delete` / `handler delete` / `route delete` / `derive schema --remove` por baixo dos panos. Cada camada tem semântica e ordem própria (DI primeiro, código depois, schema por último); cascade misturaria tudo. Responsabilidade do dev rodar a sequência completa — o erro composto guia.

### Fluxo de desmontagem típico

```
# 1. desligar do DI (em qualquer ordem entre si, mas antes de apagar pacotes)
scaffold route unregister <Domain> <Service>           # quando existir
scaffold handler unregister <Domain> <Service>
scaffold service unregister <Domain> <Verb>
scaffold repository unregister <Domain>

# 2. apagar artefatos derivados
scaffold route delete <Domain>                         # quando existir
scaffold handler delete <Domain> <Service>             # quando existir
scaffold service delete <Domain> <Verb>                # quando existir
scaffold repository delete <Domain>                    # quando existir
scaffold derive schema --remove <Domain>               # quando existir

# 3. finalmente:
scaffold domain delete <Domain>
```

Comandos marcados "quando existir" são forward refs — ainda não foram implementados, mas a mensagem de erro de `domain delete` já os sugere para guiar o futuro.

## `repository port` e `repository create` — adapter sqlx

Os dois comandos da fatia 3 entregam o **adapter sqlx** do domínio: a primeira
camada Go derivada do domain, e a base para service/handler/route nas próximas
fatias. São **independentes** — qualquer ordem é válida — e cada um falha de
forma isolada.

### `scaffold repository port <Domain> [--multi-tenant] [--table <name>]`

Patcha o arquivo do domínio em `internal/application/domain/<snake>/<snake>.go`
adicionando:

- métodos `Schema()`, `GetFilters()`, `SoftDelete()` que satisfazem a constraint
  `base_repository.Domain`;
- setter `SetFilters(*filters.Filters)`;
- campo não-exportado `filters *filters.Filters` ao final da struct;
- interface `Repository` embedando `base_repository.BaseRepository[<Domain>]`;
- imports necessários (`providers/filters`, `repositories/base_repository`).

Com `--multi-tenant` adiciona também o campo `client string`, o setter
`SetClient(string)` e o prefixo `client + "."` em `Schema()`.

Nome da tabela: default `snake_case(Domain) + "s"`, override via `--table`.

**Idempotência:** o comando checa todos os elementos antes de mutar e falha
em qualquer duplicado — `Schema` pré-existente, `filters` pré-existente,
`Repository` pré-existente, etc. Em caso de falha, o arquivo não é
modificado. Re-rodar exige remover manualmente o que conflita.

### `scaffold repository unport <Domain>` — desfaz o port

Operação inversa de `repository port`. Edita o arquivo do domínio via AST
removendo tudo que `port` enxertou:

- métodos `Schema`, `GetFilters`, `SoftDelete`, `SetFilters`;
- campo não-exportado `filters`;
- interface `Repository`;
- em multi-tenant: campo `client` e método `SetClient`;
- imports residuais sem uso (preservados quando outra função no arquivo ainda
  os usa).

**Multi-tenant é auto-detectado** — sem flag. Presença simultânea de `client`
e `SetClient` ativa o modo multi-tenant; presença de só um dos dois é erro
explícito de "estado inconsistente".

**Idempotência:** falha se qualquer elemento esperado estiver ausente
(`Schema`, `filters`, `Repository`, etc.). Em caso de falha, o arquivo não é
modificado. Re-rodar `unport` num domínio já não-portado é erro intencional —
o comando não é no-op.

**Interface `Repository` com métodos custom:** o caso real `usage_record`
embeda `BaseRepository[UsageRecord]` mas adiciona métodos custom
(`UpsertBatch`, `Summarize`). `unport` remove a interface **inteira** e emite
um aviso no stderr da CLI:

```
aviso: interface Repository tinha métodos custom — foi removida inteira;
tipos downstream que dependiam dela quebrarão a compilação
```

O dev re-decide o que fazer com a parte custom (porta de novo + reintroduzir
métodos à mão, ou reescrever o repository do zero).

Único flag: `--root` (raiz do repositório, default = diretório atual).

### `scaffold repository create <Domain>`

Cria `internal/repositories/<snake>/<snake>.go`:

```go
package <snake>_repository

import (
    "github.com/jmoiron/sqlx"

    "zord/internal/application/domain/<snake>"
    "zord/internal/repositories/base_repository"
)

type <Domain>Repository struct {
    *base_repository.BaseRepo[<snake>.<Domain>]
}

func New<Domain>Repository(mysql *sqlx.DB) *<Domain>Repository {
    return &<Domain>Repository{
        BaseRepo: base_repository.NewBaseRepository[<snake>.<Domain>](mysql),
    }
}
```

Exige que o arquivo do domínio exista e contenha a struct `<Domain>` (o repo
concreto importa o tipo). Falha se o arquivo do repositório já existe — métodos
customizados (ex.: `UpsertBatch`) ficam para o desenvolvedor adicionar à mão
depois.

### Exemplo de uso ponta a ponta

```bash
# escrever o domínio
go run ./cmd/scaffold domain create Project
go run ./cmd/scaffold field add Project ID string --tag-db=id --tag-db-pk --tag-validate=required,uuid
go run ./cmd/scaffold field add Project Name string --tag-db=name --tag-db-size=120

# derivar o adapter sqlx
go run ./cmd/scaffold repository port Project
go run ./cmd/scaffold repository create Project

# criar use cases (cada verbo em sua própria pasta)
go run ./cmd/scaffold service create Project Create
go run ./cmd/scaffold service create Project Get

# adapter HTTP — um handler 1:1 por service (pasta + arquivo gerados por verbo)
go run ./cmd/scaffold handler create Project Create
go run ./cmd/scaffold handler create Project Get

# (próximas fatias entregarão route/wire)
```

### Limitações conhecidas (fatia 3)

- O escopo é **um repository por domínio.** Sem fragmentação por agregado/contexto.
- `SoftDelete()` retorna `"deleted_at"` por padrão; sem flag de override (toda
  tabela atual usa essa coluna).
- Métodos custom no repository (ex.: `UpsertBatch` em `usagerecord`) ficam
  fora do scaffold — sempre escritos à mão depois.
- Migração dos repositórios existentes que destoam do padrão
  (`organization/ports.go`, pacote `usagerecordRepository`) é task de cleanup
  separada.

## `service create` — esqueleto do use case

`scaffold service create <Domain> <Verb>` cria
`internal/application/services/<snake_domain>/<snake_verb>/service.go` com o
esqueleto compilável do use case, seguindo o padrão usecase-per-folder de
`internal/application/services/auth/login/`:

```go
// Package <snake_verb> implementa o use case <Verb>.
package <snake_verb>

import (
    "context"

    "zord/internal/application/services"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "<lowerCamelVerb>Service"

// Input agrega os parâmetros de entrada do use case.
type Input struct{}

// Output agrega o resultado do use case.
type Output struct{}

// Service executa o use case <Verb>.
type Service struct {
    services.BaseService
}

// NewService constrói o Service com suas dependências.
func NewService(logger services.Logger, idCreator services.IdCreator) *Service {
    return &Service{
        BaseService: services.BaseService{Logger: logger, Ulid: idCreator},
    }
}

// Execute roda o use case.
func (s *Service) Execute(_ context.Context, _ Input) (*Output, *services.Error) {
    return &Output{}, nil
}
```

**Foco estrutural.** O constructor recebe apenas `logger` e `idCreator` — nenhum
port do domínio é injetado automaticamente; o dev edita o constructor à mão para
adicionar dependências (`Repository`, outros services, etc.). A assinatura de
`Execute` é fixa: `(ctx, Input) (*Output, *services.Error)`.

**Validações:**

- `<Domain>` e `<Verb>` precisam ser PascalCase exportáveis.
- O arquivo do domínio (`internal/application/domain/<snake_domain>/<snake_domain>.go`)
  precisa existir e conter a struct `<Domain>`.
- A pasta do verbo dentro do domínio ainda não pode existir — falha em
  duplicado, sem mutar nada.

**`service_test.go` não é gerado.** Cobertura é responsabilidade do dev, na
mesma task (a regra de 100% em `internal/application/services/` continua valendo).

### Limitações conhecidas (fatia 4)

- Constructor sempre minimalista (`logger`, `idCreator`). Sem flags para
  injetar `Repository` ou outros ports automaticamente.
- Sem registro no `pkg/registry` — `RegistryKey` é apenas declarado; o wire-up
  no container DI continua manual.
- Sem geração de testes — `service_test.go` é responsabilidade do dev.

## `handler create` — adapter HTTP 1:1 por service (eager deps)

A fatia 5 entrega o **adapter HTTP** no padrão **1 handler por service**: para
cada use case em `services/<d>/<s>/` existe um arquivo dedicado em
`handlers/<d>/<s>/handler.go` com responsabilidade única e método único `Handle`.
A simetria é total com o scaffold de service (NAVE-58); a fatia 6 (NAVE-63)
consome esse layout pra registrar rotas via `route add <D> <S>`.

A partir da NAVE-77 o constructor **resolve as dependências eager**: o
`registry.Resolve` roda no `New<Pascal>Handler`, guarda o service tipado em
campo do struct e o `Handle` apenas consome `h.svc.Execute(...)`. Falha de
resolução quebra `Setup()` no boot — não no primeiro request que toca a rota.

> **Histórico.** A fatia substitui a NAVE-61 (1 handler por domínio com N
> métodos via `handler method add`); aquele comando foi removido. A NAVE-70
> entregou o shape 1:1 com resolve **lazy** dentro do `Handle`; a NAVE-77
> migrou para o padrão **eager** descrito abaixo. Handlers em produção
> (`AuthHandler`, `OrgHandler`) não foram migrados para 1:1; cleanup é task
> separada.

### `scaffold handler create <Domain> <Service>`

Cria `cmd/http/handlers/<snake_domain>/<snake_service>/handler.go`:

```go
// Package <snake_service> expõe o handler HTTP do use case <Service>.
package <snake_service>

import (
    "net/http"

    "zord/cmd/http/httperr"
    "zord/internal/application/services/<snake_domain>/<snake_service>"
    "zord/pkg/registry"

    "github.com/labstack/echo/v4"
)

// RegistryKey identifica o *<Pascal>Handler no pkg/registry.
const RegistryKey = "<lowerCamel>Handler"

// <Pascal>Handler atende o use case <Service>. Mantém as deps já resolvidas pelo New.
type <Pascal>Handler struct {
    svc *<snake_service>.Service
}

// New<Pascal>Handler resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).
func New<Pascal>Handler(reg *registry.Registry) *<Pascal>Handler {
    svc := registry.Resolve[*<snake_service>.Service](reg, <snake_service>.RegistryKey)
    return &<Pascal>Handler{svc: svc}
}

// Handle executa o use case <Service>.
func (h *<Pascal>Handler) Handle(c echo.Context) error {
    var input <snake_service>.Input
    if err := c.Bind(&input); err != nil {
        return httperr.RespondBadRequest(c, err.Error())
    }
    out, svcErr := h.svc.Execute(c.Request().Context(), input)
    if svcErr != nil {
        return httperr.RespondServiceError(c, svcErr)
    }
    return c.JSON(http.StatusOK, out)
}
```

**Validações** (todas obrigatórias, falham sem mutar disco):

- `<Domain>` e `<Service>` precisam ser PascalCase exportáveis.
- O arquivo do domínio (`internal/application/domain/<snake_domain>/<snake_domain>.go`)
  precisa existir e conter a struct `<Domain>` (mesma checagem das fatias 3 e 4).
- O arquivo do service (`internal/application/services/<snake_domain>/<snake_service>/service.go`)
  precisa existir e conter `const RegistryKey` + `func NewService` — o `Handle`
  gerado importa esses símbolos por nome.
- A pasta do handler ainda não pode existir — falha em duplicado.

### Decisões fechadas (fatia 5, padrão 1:1 + eager)

- **1 handler por service, não por domínio.** Diff em uma rota não toca o
  arquivo de outra; PRs paralelos no mesmo domínio não colidem.
- **Método único `Handle`.** Sem `ServeHTTP`, sem o nome do service no método.
  Route da fatia 6 referencia `<service>Handler.Handle` sem ambiguidade.
- **Resolve eager no constructor (NAVE-77).** `registry.Resolve` roda em
  `New<Pascal>Handler` e armazena o service tipado em `h.svc`. Custo zero
  por request, struct auto-documentada (campos tipados explícitos), falha
  rápida no boot se algum service estiver ausente do registry. A ordem
  topológica em `bootstrap/setup.go` (`pkg → repositories → services →
  handlers`) já garante segurança.
- **Sem DTOs locais.** O handler binda direto em `<service>.Input` e devolve `out`
  do `Execute` direto via `c.JSON`. O service é a fonte única do shape do
  payload; handler não duplica.
- **Status code default = `http.StatusOK`.** Trocar pra 201/204/etc. é edição
  manual de uma linha. Sem flag `--status`.
- **Tratamento de erro padronizado.** `httperr.RespondBadRequest` no bind e
  `httperr.RespondServiceError` no erro do service — padrão de
  `cmd/http/handlers/auth/handler.go`. O map literal de `org/handler.go` é
  exceção legada e não é replicada.
- **Imports bare, sem alias.** Package do service é único por pasta — não há
  colisão de nome possível dentro do arquivo gerado.
- **Domain estrito.** Bate exatamente com domain de dados. Service cruzando
  domains escolhe o domain "principal" — decisão do dev, não do scaffold.
- **Constructor signature inalterada entre NAVE-70 e NAVE-77.**
  `New<Pascal>Handler(reg *registry.Registry) *<Pascal>Handler` segue
  idêntico — só o body muda. Wire-up em `bootstrap/handlers.go` (NAVE-73)
  funciona independente desta fatia.

### Limitações conhecidas (fatia 5)

- **Sem geração de anotações Swagger** (`@Summary`, `@Tags`, etc.). Continuam
  manuais.
- **Sem registro no `pkg/registry`** — `RegistryKey` é apenas declarado; o
  wire-up no container DI continua manual (analogia a NAVE-60 pra services; a
  fatia 9 cobrirá isso).
- **Sem registro de rota** — `declarable.go` continua editado à mão. A fatia 6
  (NAVE-63) cobrirá isso, já consumindo o path canônico
  `handlers/<d>/<s>/handler.go`.
- **Handlers existentes em produção (`AuthHandler`, `OrgHandler`) não migrados.**
  Cleanup é task separada.

## `route create` e `route add` — registro HTTP por domain

A fatia 6 fecha a cadeia `domain → service → handler → route`. Cada domain
tem um único arquivo de rotas (`cmd/http/routes/<snake>.go`) com um struct
`<Pascal>Route` cujos campos são os handlers 1:1 que ele expõe. Cada rota
dentro do arquivo aponta para o `Handle` do handler 1:1 do service
correspondente.

A partir da NAVE-74, o constructor da Route recebe **apenas**
`*registry.Registry` e resolve os handlers internamente (eager): cada
`route add` anexa uma atribuição
`<lowerCamel>Handler: registry.Resolve[*<svc>.<Pascal>Handler](reg, <svc>.RegistryKey)`
ao `CompositeLit` retornado pelo constructor. A assinatura
`New<Pascal>Route(reg *registry.Registry)` é **invariante** — não muda
quando services são adicionados ou removidos. Em `declarable.go` cada
domain entra como uma única linha `"<snake>": New<Pascal>Route(reg)`,
sem imports de handler e sem linhas de Resolve por service.

> **Legado.** Os arquivos atuais em `cmd/http/routes/` (`auth.go`, `org.go`,
> `health.go`) usam o padrão antigo "1 handler por domain com N métodos" e
> não são referência. O shape canônico é o que o par `route create` +
> `route add` emite. Cleanup desses arquivos é task separada.

### `scaffold route create <Domain>`

Cria `cmd/http/routes/<snake_domain>.go` com o esqueleto vazio:

```go
package routes

import (
    "zord/pkg/registry"

    "github.com/labstack/echo/v4"
)

type <Pascal>Route struct {
}

func New<Pascal>Route(reg *registry.Registry) *<Pascal>Route {
    return &<Pascal>Route{}
}

func (r *<Pascal>Route) DeclarePrivateRoutes(g *echo.Group, prefix string) {
}

func (r *<Pascal>Route) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
```

**Validações:**

- `<Domain>` precisa ser PascalCase exportável.
- O arquivo do domínio precisa existir com a struct `<Domain>`.
- O arquivo da Route ainda não pode existir — falha em duplicado, sem
  mutar nada.

### `scaffold route add <Domain> <Service> --method=<M> [--path=<p>] [--public]`

Altera `cmd/http/routes/<snake_domain>.go` via AST em **quatro pontos**,
em uma única operação atômica:

1. **Campo na struct:** `<lowerCamel>Handler *<snake_service>.<Pascal>Handler`.
2. **Atribuição no `CompositeLit` do constructor** (mantém a assinatura
   estável):

   ```go
   <lowerCamel>Handler: registry.Resolve[*<snake_service>.<Pascal>Handler](reg, <snake_service>.RegistryKey)
   ```

3. **Linha em `DeclarePrivateRoutes`** (default) ou `DeclarePublicRoutes`
   (`--public`):

   ```go
   g.<METHOD>("/"+prefix+"/<snake_domain>"+"<path>", r.<lowerCamel>Handler.Handle)
   ```

4. **Import do handler 1:1** (`zord/cmd/http/handlers/<snake_domain>/<snake_service>`).

**Flags:**

- `--method` (obrigatório): `GET|POST|PUT|PATCH|DELETE`. Case-insensitive,
  normalizado pra uppercase no output.
- `--path`: default `/<kebab-service>`. Ex.: `Login` → `/login`,
  `SelectOrg` → `/select-org`. Aceita override (ex.: `--path=/`).
- `--public`: default false. Sem flag → registra em `DeclarePrivateRoutes`.

**Validações:**

- `<Domain>` e `<Service>` em PascalCase exportável.
- O arquivo da Route existe e tem struct `<Pascal>Route`, construtor
  `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route` com assinatura
  canônica exata, e ambas as funções `Declare*Routes`. Constructor com
  parâmetros extra ou ausência do `reg` (Route hand-editada) falha com
  mensagem clara apontando para registro manual.
- O service existe (`services/<snake_domain>/<snake_service>/service.go`
  com `const RegistryKey` + `func NewService`).
- O handler 1:1 existe (`handlers/<snake_domain>/<snake_service>/handler.go`
  com struct `<Pascal>Handler` e método `Handle(echo.Context) error`).
- A rota ainda não foi registrada — comparação estrutural por nome do
  campo do handler. Tentar registrar o mesmo handler duas vezes (mesmo em
  `Declare*` opostos) falha.

### Exemplo de uso ponta a ponta

```bash
# cadeia completa do domain ao registro da rota
go run ./cmd/scaffold domain create Project
go run ./cmd/scaffold field add Project ID string --tag-db=id --tag-db-pk
go run ./cmd/scaffold field add Project Name string --tag-db=name

go run ./cmd/scaffold service create Project Create
go run ./cmd/scaffold handler create Project Create

go run ./cmd/scaffold route create Project
go run ./cmd/scaffold route add Project Create --method=POST --public
```

Resultado em `cmd/http/routes/project.go`:

```go
package routes

import (
    "zord/cmd/http/handlers/project/create"
    "zord/pkg/registry"

    "github.com/labstack/echo/v4"
)

type ProjectRoute struct {
    createHandler *create.CreateHandler
}

func NewProjectRoute(reg *registry.Registry) *ProjectRoute {
    return &ProjectRoute{
        createHandler: registry.Resolve[*create.CreateHandler](reg, create.RegistryKey),
    }
}

func (r *ProjectRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
}

func (r *ProjectRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {
    g.POST("/"+prefix+"/project"+"/create", r.createHandler.Handle)
}
```

### `scaffold route remove <Domain> <Service> [--force]`

Operação inversa do `route add`. Altera
`cmd/http/routes/<snake_domain>.go` via AST removendo, em uma única
operação atômica, os **quatro pontos** registrados pelo `route add`:

1. **Campo na struct** `<lowerCamel>Handler`.
2. **KV no `CompositeLit` do ctor** com `<lowerCamel>Handler: registry.Resolve[...]`.
3. **Linha em `DeclarePrivateRoutes` ou `DeclarePublicRoutes`** —
   auto-detectado pela presença de `r.<lowerCamel>Handler.Handle`.
4. **Import do handler 1:1** — só some se nenhum outro `SelectorExpr` no
   arquivo ainda referenciar o pkg ident (cobre arquivos hand-editados
   com múltiplas rotas do mesmo service).

Identificação por par único `(Domain, Service)` — o nome do campo
`<lowerCamel>Handler` é único por construção do `route add`, então não
há ambiguidade de método ou path.

**Flags:**

- `--force`: default false. Sem flag, comportamento é **atômico**: se
  qualquer um dos 4 pontos faltar, falha com mensagem específica e
  disco intocado. Com `--force`, remove o que existir dos 4 pontos e
  ignora os que faltarem — útil pra retry após falha intermediária ou
  pra limpar estado parcial gerado por hand-edit.
- `--root`: raiz do repositório (default: diretório atual).

**Validações:**

- `<Domain>` e `<Service>` em PascalCase exportável.
- O arquivo da Route existe e tem struct `<Pascal>Route`, ctor
  `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route` com
  **assinatura canônica exata** e ambas as funções `Declare*Routes`.
  Ctor hand-editado falha **mesmo com `--force`** — sem o shape
  canônico não dá pra localizar o `CompositeLit` com segurança.

**Esvaziamento da struct:** se ficar 0 KVs no `CompositeLit` (último
handler removido), o `gofmt` colapsa pra `&<Pascal>Route{}` numa única
linha.

**Idempotência negativa:** rodar duas vezes em sequência falha na
segunda — sem `--force`, o pré-check vê que campo/KV/stmt já não existem.

### `scaffold route register <Domain>`

Fecha a cadeia. Patcha `cmd/http/routes/declarable.go` via AST anexando ao
map retornado por `GetRoutes` a entrada:

```go
"<snake_domain>": New<Pascal>Route(reg),
```

A entrada vai para o fim do `CompositeLit` — entradas legadas (`health`,
`auth`, `org`) e quaisquer já registradas por execuções anteriores ficam
intocadas. Nenhum import ou linha de `Resolve` é adicionado: cada Route
gerada pelo shape NAVE-74 resolve os próprios handlers internamente via
`*registry.Registry`.

**Validações:**

- `<Domain>` em PascalCase exportável.
- `cmd/http/routes/<snake>.go` existe com struct `<Pascal>Route` e
  constructor `New<Pascal>Route(reg *registry.Registry) *<Pascal>Route`.
  Reusa o mesmo `assertCtorSignature` do `route add` — single source of
  truth do shape NAVE-74.
- `cmd/http/routes/declarable.go` existe e termina em
  `return map[string]Declarable{...}` (com `GetRoutes(reg *registry.Registry)`).
- Constructor com parâmetros além de `reg *registry.Registry` falha — Route
  hand-editada. Sem flag `--force`; resolver via registro manual.
- A chave `"<snake_domain>"` ainda não está presente no map — idempotente:
  re-executar para o mesmo domain sempre falha.

**Exemplo seguindo o domain `Project` do exemplo anterior:**

```bash
go run ./cmd/scaffold route register Project
```

`cmd/http/routes/declarable.go` antes:

```go
return map[string]Declarable{
    "health": NewHealthRoute(),
    "auth":   NewAuthRoute(authH, jwtValidator),
    "org":    NewOrgRoute(orgH),
}
```

Depois:

```go
return map[string]Declarable{
    "health":  NewHealthRoute(),
    "auth":    NewAuthRoute(authH, jwtValidator),
    "org":     NewOrgRoute(orgH),
    "project": NewProjectRoute(reg),
}
```

Uma linha adicionada. Nenhum import, nenhum Resolve novo. `go build ./...`
compila e `go vet ./...` fica limpo.

### `scaffold route unregister <Domain>`

Operação inversa de `route register` (NAVE-91). Patcha
`cmd/http/routes/declarable.go` via AST removendo do map retornado por
`GetRoutes` a entrada:

```go
"<snake_domain>": New<Pascal>Route(reg),
```

Outras entradas (legadas — `health`, `auth`, `org` — e quaisquer outras
registradas via `route register`) ficam intocadas. Nenhum import é removido:
o ctor da Route não exige import externo além do pacote `routes` interno.

**Não é inversa de `route create`** — o arquivo `cmd/http/routes/<snake>.go`
da Route NÃO é apagado. Para isso, use `route delete` (simétrico a
`route create`). Útil também para limpar entradas órfãs (chaves cujo arquivo
da Route já foi apagado à mão).

**Validações:**

- `<Domain>` em PascalCase exportável.
- `cmd/http/routes/declarable.go` existe e termina em
  `return map[string]Declarable{...}` (com `GetRoutes(reg *registry.Registry)`).
- A chave `"<snake_domain>"` está presente no map — idempotência negativa:
  re-executar para o mesmo domain sempre falha após sucesso anterior.

**Exemplo (desfazendo o registro do domain `Project` do exemplo anterior):**

```bash
go run ./cmd/scaffold route unregister Project
```

`cmd/http/routes/declarable.go` antes:

```go
return map[string]Declarable{
    "health":  NewHealthRoute(),
    "auth":    NewAuthRoute(authH, jwtValidator),
    "org":     NewOrgRoute(orgH),
    "project": NewProjectRoute(reg),
}
```

Depois:

```go
return map[string]Declarable{
    "health": NewHealthRoute(),
    "auth":   NewAuthRoute(authH, jwtValidator),
    "org":    NewOrgRoute(orgH),
}
```

### `scaffold route delete <Domain>` (NAVE-99)

Inverso disciplinado de `route create`. Apaga `cmd/http/routes/<snake_domain>.go`
quando o estado está limpo — sem `--force`, sem `--ignore-missing`.

**Guardas (todas obrigatórias, falham sem mutar disco):**

- `<Domain>` em PascalCase exportável.
- `cmd/http/routes/<snake_domain>.go` existe. Se já foi removido, falha
  com `arquivo de rotas não existe: ...` — idempotência inversa.
- `cmd/http/routes/declarable.go` (se existir) **não** tem mais a chave
  `"<snake_domain>"` no map de `GetRoutes`. Se ainda tiver, remova a entrada
  antes (manualmente ou via `route unregister`). Se `declarable.go`
  não existir, passa por vacuidade — repos novos ou em scaffold.
- A struct `<Pascal>Route` está vazia (sem handlers anexados via
  `route add`). Se houver campos, o erro lista os nomes e aponta edição
  manual — use `route remove <Domain> <Service>` (NAVE-92) para remover
  individualmente.

A guarda do `declarable.go` reusa `findFreeFuncDecl`, `findRoutesMapLit` e
`hasRouteEntry` de `route_register.go` — single source of truth da
validação do registro.

**Exemplo:**

```bash
go run ./cmd/scaffold route create Foo
go run ./cmd/scaffold route delete Foo
# removido: cmd/http/routes/foo.go
go run ./cmd/scaffold route delete Foo
# scaffold: arquivo de rotas não existe: cmd/http/routes/foo.go
```


### Decisões fechadas (fatia 6 + NAVE-74)

- **Constructor recebe só `*registry.Registry`.** Assinatura imutável,
  sem variar com o número de services. Cresce em features sem mudar
  contrato — a única mudança é o conteúdo do `CompositeLit` retornado.
- **Resolve eager no constructor.** Falha rápido no bootstrap se algum
  handler estiver ausente do registry, não na primeira request. `Declare*`
  fica limpo (só `g.<METHOD>(...)`).
- **`<Domain>` é fonte única.** Determina onde o service vive, de onde o
  handler é importado e qual arquivo de rotas editar. Sem flag de
  desambiguação, sem cross-domain.
- **Default private, flag única `--public`.** Sem `--auth-user-scope`,
  sem composição de middlewares custom. Quando uma rota precisar, o
  dev edita à mão depois do `route add` — mas qualquer `route add`
  subsequente falha com mensagem clara (assinatura canônica do constructor
  perdida).
- **Detecção de duplicata por estrutura.** Um handler 1:1 = no máximo uma
  rota. Re-rodar `route add` idêntico falha porque o campo já existe.
- **AST puro.** Mesmo padrão das fatias anteriores — `astbuild` para
  construir o esqueleto inicial; `astutil.AddImport` + manipulação direta
  do struct/ctor/body em `route add`.

### Limitações conhecidas (fatia 6 + NAVE-74 + NAVE-65)

- **Sem suporte a middlewares no constructor.** Quem precisar (JWT
  validator, RBAC, user-scope, rate-limit) edita à mão; aí o `route add`
  passa a falhar e a manutenção segue manual. Issues separadas tratam
  esse caso.
- **Sem geração de anotações Swagger** (`@Summary`, `@Router`). Continua
  manual.
- **Sem comando `route list`** — sem demanda. `route remove` foi
  implementado em NAVE-92.

> Atualização NAVE-91: `route unregister <Domain>` (inverso de
> `route register`) já existe — ver seção acima.
- **Cross-domain não suportado.** Se aparecer recorrente, vira flag
  dedicada em issue futura.

## `projection create` e `projection field add/remove` — structs agregadas no domain

A fatia 12 (NAVE-87) cobre o pattern recorrente observado em produção
(`UsageRecord.ResourceSummary`/`Summary`): **projection structs** que
são tipos de retorno de queries customizadas com `GROUP BY`/`SUM`/
agregação. Convivem com o domain principal no mesmo arquivo, com tag
`json` (resposta API) sempre presente e tag `db` (StructScan do sqlx)
decidida por campo.

Espelha o padrão atômico `domain create` + `field add` — 1 comando = 1
artefato. Os três comandos são **independentes** e cada um falha de
forma isolada.

### `scaffold projection create <Domain> <ProjectionName>`

Patcha `internal/application/domain/<snake>/<snake>.go` anexando ao
final do arquivo:

```go
type <ProjectionName> struct {
}
```

Sem fields, sem comentário doc — exatamente como `domain create` emite
a casca vazia do domain principal.

**Validações** (todas falham sem mutar disco):

- `<Domain>` e `<ProjectionName>` em PascalCase exportável.
- Arquivo do domain existe com a struct `<Domain>`.
- `<ProjectionName>` ≠ `<Domain>` (colisão).
- Nenhum tipo top-level (struct, interface, alias) com nome
  `<ProjectionName>` no arquivo — falha em duplicado.

### `scaffold projection field add <Domain> <ProjectionName> <Field> <Type> [flags]`

Anexa um campo ao `StructType` da projection alvo:

```go
type <ProjectionName> struct {
    <Field> <Type> `[db:"<col>" ]json:"<snake_field>"`
}
```

**Flags:**

| Flag | Default | Comportamento |
|---|---|---|
| `--tag-db=<col>` | snake do nome do campo | Override do nome da coluna `db`. |
| `--no-db-tag` | — | Suprime a tag `db`. Pra campos de struct composta que não vêm de `StructScan` (ex.: `Total float64` em `Summary`). Mutuamente exclusivo com `--tag-db`. |

Tag `json` é sempre presente, com valor = snake do nome do campo.

**Tipos aceitos em `<Type>`:**

- Escalares Go: `string`, `int`, `int32`, `int64`, `float64`, `bool`, `time.Time` (e ponteiros: `*string`, `*time.Time`).
- Slice de outra projection do mesmo domain: `[]<OutraProjection>`.
- Ponteiro de outra projection do mesmo domain: `*<OutraProjection>`.

Slices e structs nomeadas nunca recebem `db` automaticamente (sqlx não
popula esses tipos sem custom scanner) — mesmo sem `--no-db-tag`.
Referências a outras projections (`[]X`, `*X`) exigem que `X` exista
como tipo top-level no mesmo arquivo do domain — criar primeiro a "raw",
depois a "composta".

**Validações:**

- `<Domain>`, `<ProjectionName>`, `<Field>` em PascalCase exportável.
- Arquivo do domain existe, contém struct `<Domain>` e contém
  `type <ProjectionName> struct` (criada via `projection create`).
- `<Type>` parseia como expressão Go válida; pacotes externos no allowlist
  (mesma checagem do `field add` — atualmente só `time`).
- Falha em duplicado: campo já existente na projection → erro, arquivo
  intacto.
- `--tag-db` e `--no-db-tag` mutuamente exclusivos.

### `scaffold projection field remove <Domain> <ProjectionName> <Field>`

Simétrico ao `field remove` do domain — apaga o campo da projection.
Imports que ficarem sem uso são removidos. Falha se a projection ou o
campo não existirem.

### Exemplo ponta a ponta — reproduz o pattern do NAVE-9

```bash
# domain raiz
go run ./cmd/scaffold domain create UsageRecord
go run ./cmd/scaffold field add UsageRecord ID string --tag-db=id --tag-db-pk
go run ./cmd/scaffold field add UsageRecord Namespace string --tag-db=namespace
go run ./cmd/scaffold field add UsageRecord Cost float64 --tag-db=cost

# projection raw (vinda direto de GROUP BY)
go run ./cmd/scaffold projection create UsageRecord ResourceSummary
go run ./cmd/scaffold projection field add UsageRecord ResourceSummary ResourceType string
go run ./cmd/scaffold projection field add UsageRecord ResourceSummary Unit string
go run ./cmd/scaffold projection field add UsageRecord ResourceSummary Quantity float64
go run ./cmd/scaffold projection field add UsageRecord ResourceSummary Total float64

# projection composta (populada em Go, sem db tags exceto onde indicado)
go run ./cmd/scaffold projection create UsageRecord Summary
go run ./cmd/scaffold projection field add UsageRecord Summary Namespace string --no-db-tag
go run ./cmd/scaffold projection field add UsageRecord Summary PeriodStart string --no-db-tag
go run ./cmd/scaffold projection field add UsageRecord Summary PeriodEnd string --no-db-tag
go run ./cmd/scaffold projection field add UsageRecord Summary Total float64 --no-db-tag
go run ./cmd/scaffold projection field add UsageRecord Summary ByResource []ResourceSummary
```

Resultado em `internal/application/domain/usage_record/usage_record.go`:

```go
type ResourceSummary struct {
    ResourceType string  `db:"resource_type" json:"resource_type"`
    Unit         string  `db:"unit" json:"unit"`
    Quantity     float64 `db:"quantity" json:"quantity"`
    Total        float64 `db:"total" json:"total"`
}

type Summary struct {
    Namespace   string            `json:"namespace"`
    PeriodStart string            `json:"period_start"`
    PeriodEnd   string            `json:"period_end"`
    Total       float64           `json:"total"`
    ByResource  []ResourceSummary `json:"by_resource"`
}
```

A partir daí o método custom no repository (`Summarize`, `UpsertBatch`,
etc.) que consome a projection continua escrito **à mão** — esta fatia
cobre apenas o lado do domain.

### Decisões fechadas (fatia 12)

- **Padrão atômico `create` + `field add`.** Espelha o scaffold de
  domain (NAVE-56). 1 comando = 1 artefato; sem mini-DSL com `--field`
  repetível.
- **Mesmo arquivo do domain.** Projection é semanticamente do domain;
  não merece arquivo próprio (e é o pattern observado em produção).
- **`json` sempre, `db` por field.** Sem flag global na criação — o
  estado de cada campo fica visível no comando que o adiciona.
- **Slices e structs nomeadas nunca ganham `db` automaticamente.**
  `sqlx.StructScan` não popula esses tipos sem custom scanner —
  forçar `db` aí gera erro silencioso em runtime.
- **Sem geração do método no Repository.** Esta fatia cobre só o domain;
  `Summarize`/`UpsertBatch` etc. continuam manuais (issue separada se
  recorrer).
- **Sem suporte a ports externas (`AllocationSource`).** Pattern
  diferente — issue separada.

### Limitações conhecidas (fatia 12)

- **Sem geração do método no Repository.** A projection é só o tipo de
  retorno; o `SELECT ... GROUP BY ...` que popula a projection continua
  escrito à mão no repository concreto.
- **Sem suporte a ports externas (`AllocationSource`, etc.).** Sources
  externos (metering, S3, etc.) ficam fora desta fatia.
- **Sem `projection remove <Name>`.** Simétrico à falta de
  `domain remove` no scaffold — apagar projection é edição manual.
- **Sem `--tag-validate` ou outras tags.** Projections são saída
  (resposta API), não input — validação não se aplica.
- **Auto-referência via `*<ProjectionName>` é aceita.** Não cria ciclo
  de tipo no Go, mas exige cautela do dev (pra evitar JSON cíclico).

## Build

```bash
go build ./cmd/scaffold
```

O binário resultante (`./scaffold`) opera sobre a raiz do repositório alvo.

## Estrutura interna

Todo o código do scaffold vive em `tools/scaffold/` num único pacote `scaffold` (layout flat — sem subpacotes `cmd/` ou `internal/`, NAVE-83). Convenção de arquivos:

- `root.go` — `NewRootCmd()`, monta o tree Cobra
- `cmd_<área>.go` — subcomandos Cobra (`cmd_domain.go`, `cmd_field.go`, `cmd_derive.go`, etc.)
- `<área>.go` — geradores por área (`domain.go`, `field.go`, `schema.go`, etc.); quando há múltiplos arquivos por área, prefixo: `schema_builder.go`, `route_create.go`, `register_repository.go`, etc.
- `name.go`, `astbuild.go` — helpers utilitários

Entrypoint do binário continua sendo `cmd/scaffold/main.go` na raiz do repo (chama `scaffold.NewRootCmd().Execute()`).

Funções exportadas usam prefixo da área pra evitar colisões no namespace único: `DomainCreate`, `FieldAdd`, `SchemaDerive`, `RepositoryPort`, `RouteRegister`, `RegisterHandler`, etc.
