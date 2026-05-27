# Zord Microframework
## Build your mecha

> **Tip for MCP-aware clients (Cursor, Claude, etc.):**
> To ensure code quality and automatic validations, add the following rule to your client's **User rules**:
>
> **after each code update, run the `arch_analyze` MCP tool**
>
> This way, every code change is automatically validated by the architecture analysis tool.

GOlang base repository with an AST-driven code generator to bootstrap a fast Golang project based on hexagonal architecture.

---

# Development
> Remember to create your .env file based on .env.example

### 1. Using Docker Compose
Up mysql and zord project:

``` SHELL
docker compose up
```

The compose stack joins an external network named `zord-network`.

<br />

#### 2. Using raw go build

You will need to build the http/main.go file:

``` SHELL
go build -o server cmd/http/main.go
```

Then run the server

``` SHELL
./server
```

<br />

#### 3. Running from go file

``` SHELL
go run cmd/http/main.go
```

<br />

**Note:** To run the local build as described in the second or third option, a MySQL server must be running. This is necessary for the application to interact with its database. The easiest way to set up a MySQL server locally is by using Docker. Below is a command to start a MySQL server container using Docker:

``` SHELL
docker compose up mysql -d
```
This command will ensure that a MySQL server is running in the background, allowing you to execute the local build successfully.

---

### CLI

#### build cli

to build cli into binary file run
``` SHELL
go build -o cli cmd/cli/main.go
```

then you can run all cli commands with the binary file
``` SHELL
./cli -h
```

if you're developing something in the cli the best way is to run it directly so all changes are picked up
``` SHELL
go run ./cmd/cli -h
```

---

#### CLI Commands

Run architecture analysis over all domains (lint):
``` SHELL
go run ./cmd/cli arch-analyse
```

Inspect the database from the HCL description:
``` SHELL
go run ./cmd/cli inspect
```

Migrate the database from the HCL description:
``` SHELL
go run ./cmd/cli migrate
```

Generate an HCL schema from an existing database:
``` SHELL
go run ./cmd/cli generate-schema-from-db <schema name> <database name>
```

---

### Code generation (scaffold)

Code generation is done by the `scaffold` binary — an **AST-pure**, layer-by-layer generator. It never templates whole files blindly; it edits the Go AST of the target layer so the output stays compilable and idempotent.

``` SHELL
go run ./cmd/scaffold --help
```

The generator is organized in command groups, one per architectural layer:

| Group | Subcommands | Purpose |
|---|---|---|
| `domain` | `create`, `delete` | Domain struct under `internal/application/domain/<name>/` |
| `field` | `add`, `remove` | Granular fields on a domain struct |
| `derive` | `schema` | (Re)generate or remove the domain's Atlas HCL block |
| `repository` | `create`, `delete`, `port`, `unport`, `register`, `unregister` | sqlx repository adapter + Repository interface on the domain + DI wire-up in `bootstrap/repositories.go` |
| `service` | `create`, `delete`, `register`, `unregister` | Use-case-per-folder service + DI wire-up in `bootstrap/services.go` |
| `request` | `field`, `validator` | `request.go` Data struct fields and validation toggle of a use case |
| `response` | `field` | `response.go` Response struct fields of a use case |
| `handler` | `create`, `delete`, `register`, `unregister` | 1:1 HTTP handler for a service + DI wire-up in `bootstrap/handlers.go` |
| `route` | `create`, `add`, `remove`, `delete`, `register`, `unregister` | HTTP route file per domain + registration in `cmd/http/routes/declarable.go` |
| `projection` | `create`, `field` | Projection structs (return types for aggregate queries) |

Use `go run ./cmd/scaffold <group> --help` and `go run ./cmd/scaffold <group> <subcommand> --help` to see the exact flags for each command.

**Obs:** When generating code inside a docker container, fix file ownership afterwards with `sudo chown $USER:$USER -R .`.

---

### Dependency injection (`bootstrap/`)

The `bootstrap/` package is the single authorized wiring point of the project — `cmd`, `internal` and `pkg` never construct their dependencies elsewhere.

`bootstrap.Setup()` loads the configs and assembles the dependency graph into `pkg/registry` in **topological order**:

```
pkg → repositories → services → handlers
```

It returns the ready-to-use `*registry.Registry` and the API prefix. Each layer's `register*` function (`registerPkg`, `registerRepositories`, `registerServices`, `registerHandlers`) lives in its own file (`bootstrap/pkg.go`, `repositories.go`, `services.go`, `handlers.go`). Dependencies are looked up with the typed helper `registry.Resolve[T](reg, key)`.

The scaffold `register`/`unregister` subcommands edit these `bootstrap/*.go` files automatically — you rarely touch them by hand.

---

#### Run tests
Run all tests:
``` SHELL
go test ./...
```

Verify code coverage:
``` SHELL
// Generate coverage output
go test ./... -coverprofile=coverage.out

// Generate HTML file
go tool cover -html=coverage.out
```

---

### Development

Want to contribute? Great!

Make a change in your files and be careful with your updates.
**Any new code is only accepted with all validations passing** — run `go run ./cmd/cli arch-analyse` before opening a PR.

---

### MCP Server

`zord-mcp` is the project's Model Context Protocol server. It exposes the scaffold generator and the architecture analyser as MCP tools over **stdio** transport, so an MCP-aware client (Cursor, Claude, etc.) can generate code and lint architecture directly.

It provides **32 tools**: the `scaffold_*` family (one per scaffold subcommand — domain, field, derive, repository, service, request/response field, handler, route, projection) plus `arch_analyze`. Full per-tool documentation lives in `tools/mcp/README.md`.

#### Running the server

``` SHELL
go run ./cmd/mcp --repo .
```

`--repo` sets the target repository the tools operate on (defaults to the current working directory). Each individual tool call can also override the target via an optional `repo` argument.

#### Configuring an MCP client

The repository ships a ready-to-use client config at `zord-mcp.json`:

```json
{
  "mcpServers": {
    "zord-mcp": {
      "type": "stdio",
      "command": "go",
      "args": ["run", "./cmd/mcp", "--repo", "."],
      "env": {}
    }
  }
}
```

Point your MCP client at this config (or copy the `mcpServers.zord-mcp` entry into your client's settings). If you prefer a prebuilt binary, swap `command`/`args` for the compiled `mcp` binary:

``` SHELL
go build -o mcp ./cmd/mcp
```

```json
{
  "mcpServers": {
    "zord-mcp": {
      "type": "stdio",
      "command": "/abs/path/to/mcp",
      "args": ["--repo", "/abs/path/to/your/repo"],
      "env": {}
    }
  }
}
```

#### Smoke test

A full end-to-end smoke test that drives the server through real tool calls (create a domain + service via MCP) lives at `tools/mcp/demo.sh`.

---

### Agent Skills

`skills/` ships **harness-agnostic** agent skills — documented workflows any AI tool (Claude Code, Cursor, a custom agent) can discover and run, not just one vendor. Discovery is a single machine-readable index, `skills/manifest.json`; each skill is a `SKILL.md` written against an abstract capability vocabulary that each harness maps to its own primitives.

The first skill is `task-init`: a multi-agent orchestrator that fans out one planning sub-agent per task — a dedicated worktree per task and a human review gate before any code is written.

- Format, discovery and capability vocabulary: `skills/README.md`
- Claude Code adapter (links skills into `.claude/skills/`): `skills/adapters/claude-code/install.sh`
- Any other tool: `skills/adapters/generic.md`
