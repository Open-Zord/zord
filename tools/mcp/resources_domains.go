package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// domainsList enumera os subdiretórios de internal/application/domain/ que
// têm um arquivo Go de mesmo nome (snake_case). Retorna nomes em snake_case
// (como aparecem no FS). Reconverter para PascalCase fica com o cliente.
func domainsList(repo string) ([]string, error) {
	domainDir := filepath.Join(repo, "internal/application/domain")
	entries, err := os.ReadDir(domainDir)
	if err != nil {
		return nil, fmt.Errorf("ler %q: %w", domainDir, err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Convenção: cada domain tem um arquivo <snake>.go no diretório.
		probe := filepath.Join(domainDir, name, name+".go")
		if _, err := os.Stat(probe); err == nil {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func registerDomainsResources(s *mcpsdk.Server, repo string) {
	s.AddResource(&mcpsdk.Resource{
		Name:        "scaffold_domains",
		Title:       "Scaffold: lista de domains",
		URI:         "scaffold://domains",
		Description: "Lista (JSON) de todos os domains existentes em internal/application/domain/ (nomes em snake_case).",
		MIMEType:    "application/json",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		names, err := domainsList(repo)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(map[string]any{"domains": names})
		if err != nil {
			return nil, fmt.Errorf("marshal domains: %w", err)
		}
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(body),
			}},
		}, nil
	})

	s.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "scaffold_domain",
		Title:       "Scaffold: source de um domain",
		URITemplate: "scaffold://domain/{name}",
		Description: "Source Go do arquivo do domain (internal/application/domain/<snake>/<snake>.go). {name} é o nome em snake_case.",
		MIMEType:    "text/x-go",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		name, err := domainNameFromURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		domainFile := filepath.Join(repo, "internal/application/domain", name, name+".go")
		body, err := os.ReadFile(domainFile) //nolint:gosec // G304: path resolvido a partir do repo + segmento URI sanitizado
		if err != nil {
			if os.IsNotExist(err) {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
			return nil, fmt.Errorf("ler domain %q: %w", domainFile, err)
		}
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/x-go",
				Text:     string(body),
			}},
		}, nil
	})
}

// domainNameFromURI extrai e valida o {name} de "scaffold://domain/{name}".
// Aceita apenas snake_case ([a-z0-9_]) — bloqueia path traversal e nomes
// inválidos antes de tocar o FS.
func domainNameFromURI(uri string) (string, error) {
	const prefix = "scaffold://domain/"
	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("URI %q não casa com scaffold://domain/{name}", uri)
	}
	name := strings.TrimPrefix(uri, prefix)
	if name == "" {
		return "", fmt.Errorf("URI %q sem nome de domain", uri)
	}
	for _, r := range name {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !valid {
			return "", fmt.Errorf("nome %q inválido — apenas [a-z0-9_]", name)
		}
	}
	return name, nil
}
