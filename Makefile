# Zord — módulo único github.com/Open-Zord/zord.
# O runtime boilerplate vive versionado em internal/skeleton/runtime/ e é
# embarcado direto no binário (sem passo de geração).

.PHONY: build test vet arch-analyse

# build compila o CLI `zord` (com o runtime embarcado).
build:
	go build -o zord ./cmd/zord

# test roda toda a suíte: tooling + runtime (dogfood, compilado in-place).
test:
	go test ./...

vet:
	go vet ./...

# arch-analyse dogfooda o gate: gera um projeto efêmero a partir do skeleton
# embarcado e roda o gate nele. Não rodamos o gate direto no skeleton porque seu
# module path (github.com/Open-Zord/zord/internal/skeleton/runtime) contém
# `/internal/`, o que envenenaria as regras de isolamento baseadas nesse segmento;
# num projeto gerado o module path é limpo, como no uso real.
arch-analyse: build
	@tmp=$$(mktemp -d); \
	./zord new $$tmp/proj --module example.com/dogfood >/dev/null && \
	( cd $$tmp/proj && "$(CURDIR)/zord" arch-analyse ); \
	status=$$?; rm -rf $$tmp; exit $$status
