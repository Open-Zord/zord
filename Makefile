# Monorepo Zord — alvos de apoio.

.PHONY: embed-sync

# embed-sync materializa o runtime embarcado (cli/internal/runtimefs/_runtime) a
# partir da raiz. _runtime é gerado e gitignored, então rode isto após um clone
# limpo (e antes de buildar o cli) ou sempre que o runtime da raiz mudar.
embed-sync:
	go generate -C cli ./internal/runtimefs
