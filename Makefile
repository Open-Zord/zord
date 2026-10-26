# Monorepo Zord — alvos de apoio.

.PHONY: embed-sync embed-check

# embed-sync: regenera o runtime materializado que o `zord new` embarca.
embed-sync:
	go generate -C cli ./internal/runtimefs

# embed-check: drift-guard. _runtime precisa estar em sync com a raiz e commitado
# (é o que permite `go install` compilar o embed a partir do module zip).
embed-check: embed-sync
	@test -z "$$(git status --porcelain cli/internal/runtimefs/_runtime)" \
		|| { echo "drift no runtime embarcado: rode 'make embed-sync' e commite cli/internal/runtimefs/_runtime"; \
		     git status --porcelain cli/internal/runtimefs/_runtime; exit 1; }
	@echo "embed em sync ✓"
