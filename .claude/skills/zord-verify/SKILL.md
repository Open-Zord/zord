---
name: zord-verify
description: Run the Zord verification battery before committing a change — static checks (gofmt/vet/build/test), the architecture gate dogfooded on a freshly generated project, the full `zord new` end-to-end, and an optional Docker health smoke. Use when you've changed Zord and want to confirm it works, or when the user says "verify", "check this works", or asks to run the gate/E2E. Picks depth by what changed.
version: 1.0.0
capabilities: [run_shell, read_file]
---

Verifies a change to the Zord repo the way it actually has to hold together: the
tooling compiles and passes the gate, **and** the embedded runtime still
generates a correct, buildable, gate-clean project. Run it before every commit.

It replaces reconstructing the battery by hand each time, and encodes the
non-obvious parts — the gate can't run in-place on the skeleton, the module token
must be fully rewritten, the health path is `/v1/health`.

All commands run from the repo root. Put throwaway artifacts in your scratchpad
(or `$(mktemp -d)`), never inside the repo.

## Pick the depth

Run the tiers up to the deepest one your change touches. Higher tiers include the
lower ones.

| You changed… | Run through |
|---|---|
| Only tooling (`internal/commands`, `internal/tools`, `cmd/zord`, `projectgen`) | **Tier 2** |
| The runtime skeleton (`internal/skeleton/runtime/**`), `embed.go`, or `projectgen` | **Tier 3** |
| Runtime persistence / `docker/` / `compose.yml` / `.env.*` / schema | **Tier 4** |
| `syscall`/platform-specific code, deps, or anything build-constrained | **also Tier 5** |
| Docs only | **Tier 1** |

When unsure, run through Tier 3.

## Tier 1 — static checks (always)

```sh
gofmt -l .                 # MUST print nothing; if it lists files, they're unformatted
go vet ./...
go build ./...
go test ./...
```

`go test ./...` compiles and tests the embedded runtime in place (it's committed,
compiled source) — that's the in-repo dogfood of the runtime.

## Tier 2 — architecture gate (dogfood)

```sh
make arch-analyse          # expect all 19 validations "concluída com sucesso"
```

`make arch-analyse` generates an **ephemeral project** and runs the gate there.
Do **not** try to run `zord arch-analyse` inside `internal/skeleton/runtime/`: the
skeleton's module path is `github.com/Open-Zord/zord/internal/skeleton/runtime`,
which contains `/internal/` and poisons the gate's segment-based isolation rules.
A generated project has a clean module path, exactly like real usage.

## Tier 3 — `zord new` end-to-end

Prove the embed + projectgen + tidy + gate work for a real generated project:

```sh
ZV=$(mktemp -d); BIN=$(mktemp -d)/zord
go build -o "$BIN" ./cmd/zord
"$BIN" new "$ZV/app" --module example.com/verify     # clean, non-zord module path
cd "$ZV/app"
go build ./... && go vet ./... && go test ./...
"$BIN" arch-analyse                                   # 19/19 green
# the skeleton token must be fully rewritten — this must print 0:
grep -rc "Open-Zord/zord/internal/skeleton/runtime" . | awk -F: '{s+=$2} END{print s+0}'
cd - >/dev/null
```

Check `go.mod` says `module example.com/verify` and that `go.mod.embed` was
written out as `go.mod`.

## Tier 4 — Docker health smoke

Only when the change can affect boot/persistence. Heaviest tier (builds an image).

```sh
cd "$ZV/app"
docker rm -f zord-http zord-mysql >/dev/null 2>&1 || true
docker compose up -d --build
for i in $(seq 1 30); do
  code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:9000/v1/health || true)
  [ "$code" = "200" ] && { echo "health 200"; break; }
  sleep 2
done
docker compose down -v
cd - >/dev/null
```

Expect `200` with body `"OK"`. The app listens on `HTTP_PORT` (9000) under
`API_PREFIX` (`v1`). `POST /v1/dummy` returning 500 before `zord migrate` is
expected (the table isn't created on boot) — not a failure of this tier.

## Tier 5 — cross-platform build (optional)

When the change touches `syscall`, build tags, or new deps, confirm the binary
still cross-compiles with CGO off (what the release matrix needs):

```sh
for t in linux/amd64 linux/arm64 darwin/arm64 windows/amd64 windows/arm64; do
  CGO_ENABLED=0 GOOS=${t%/*} GOARCH=${t#*/} go build -o /dev/null ./cmd/zord \
    && echo "OK $t" || echo "FAIL $t"
done
```

## Cleanup

```sh
rm -rf "$ZV" "$BIN"; rm -f ./zord
```

## Report

Summarize concisely: which tiers ran and their result. Call out the first failure
with its command and output — do not report "verified" if any tier failed or was
skipped when the change-map said to run it. If you skipped a tier deliberately
(e.g. Docker unavailable), say so explicitly.
