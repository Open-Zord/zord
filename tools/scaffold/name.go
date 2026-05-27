// Package scaffold (área name) converte e valida identificadores usados pelo scaffold.
package scaffold

import (
	"unicode"
)

// ToSnake converte um identificador PascalCase/camelCase em snake_case.
// Grupos consecutivos de maiúsculas são tratados como uma unidade:
//
//	"Foo"          -> "foo"
//	"FooBar"       -> "foo_bar"
//	"OrgMembership" -> "org_membership"
//	"HTTPServer"   -> "http_server"
//	"UserID"       -> "user_id"
func ToSnake(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	out := make([]rune, 0, len(runes)+4)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				out = append(out, '_')
			} else if unicode.IsUpper(prev) && nextLower {
				out = append(out, '_')
			}
		}
		out = append(out, unicode.ToLower(r))
	}
	return string(out)
}

// ToLowerCamel converte um identificador PascalCase em lowerCamelCase trocando
// apenas a primeira runa pra minúscula. Para PascalCase de palavra única o
// resultado é a forma all-lower; para múltiplas palavras o resto preserva a
// capitalização:
//
//	"Login"     -> "login"
//	"SelectOrg" -> "selectOrg"
//	"Foo"       -> "foo"
//
// Não trata acronyms internos (ex.: "HTTPServer" -> "hTTPServer"); o scaffold
// não usa verbos em acronym e o caller é responsável por usar PascalCase.
func ToLowerCamel(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// IsValidExportedIdent valida que s é um identificador Go exportável:
// começa com letra maiúscula e contém apenas letras, dígitos ou underscore.
func IsValidExportedIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsUpper(r) {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
