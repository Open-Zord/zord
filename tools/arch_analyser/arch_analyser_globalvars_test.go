package arch_analyser

import (
	"strings"
	"testing"
)

// const (incluindo constant-error) é sempre permitido, em qualquer escopo.
func TestValidateNoGlobalVars_ConstIsAllowed(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/domain/org/org.go": `package org

type orgErr string

func (e orgErr) Error() string { return string(e) }

const ErrDuplicateSlug = orgErr("slug duplicado")

const MaxRetries = 5
`,
		"pkg/slug/slug.go": `package slug

const Prefix = "org-"
`,
	})
	if err := ValidateNoGlobalVars(root); err != nil {
		t.Fatalf("esperado sem violação para const, got: %v", err)
	}
}

// var global mutável é bloqueada mesmo com prefixo Err — não há mais isenção
// por nome.
func TestValidateNoGlobalVars_ErrVarBlocked(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/application/domain/org/org.go": `package org

import "errors"

var ErrDuplicateSlug = errors.New("slug duplicado")
`,
	})
	err := ValidateNoGlobalVars(root)
	if err == nil {
		t.Fatal("esperado violação para var ErrXxx global")
	}
	if !strings.Contains(err.Error(), "ErrDuplicateSlug") {
		t.Errorf("erro não menciona a var: %v", err)
	}
}

// var global qualquer (sem prefixo Err) também é bloqueada.
func TestValidateNoGlobalVars_PlainVarBlocked(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/foo/foo.go": `package foo

var contador = 0
`,
	})
	err := ValidateNoGlobalVars(root)
	if err == nil {
		t.Fatal("esperado violação para var global comum")
	}
	if !strings.Contains(err.Error(), "contador") {
		t.Errorf("erro não menciona a var: %v", err)
	}
}

// A varredura cobre pkg/ e cmd/, não só internal/.
func TestValidateNoGlobalVars_ScansPkgAndCmd(t *testing.T) {
	rootPkg := setupFakeRepo(t, map[string]string{
		"pkg/validator/validator.go": `package validator

var messages = map[string]string{"a": "b"}
`,
	})
	err := ValidateNoGlobalVars(rootPkg)
	if err == nil || !strings.Contains(err.Error(), "messages") {
		t.Fatalf("esperado violação em pkg/, got: %v", err)
	}

	rootCmd := setupFakeRepo(t, map[string]string{
		"cmd/http/foo.go": `package main

var cache = map[string]int{}
`,
	})
	err = ValidateNoGlobalVars(rootCmd)
	if err == nil || !strings.Contains(err.Error(), "cache") {
		t.Fatalf("esperado violação em cmd/, got: %v", err)
	}
}

// _test.go é ignorado: testes podem declarar vars globais à vontade.
func TestValidateNoGlobalVars_IgnoresTestFiles(t *testing.T) {
	root := setupFakeRepo(t, map[string]string{
		"internal/foo/foo_test.go": `package foo

var fixture = "x"
`,
	})
	if err := ValidateNoGlobalVars(root); err != nil {
		t.Fatalf("esperado sem violação para arquivo de teste, got: %v", err)
	}
}
