package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestValidatorSet_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")

	rel, err := RequestValidatorSet(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"})
	if err != nil {
		t.Fatalf("RequestValidatorSet: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`"zord/internal/application/services"`,
		"validator services.Validator",
		"func NewRequest(data *Data, validator services.Validator) *Request",
		"validator: validator",
		"errs := r.validator.ValidateStruct(r.Data)",
		"for _, err := range errs",
		"if err != nil",
		"return err",
		"return nil",
	)
	if err := parseGoSrc([]byte(got)); err != nil {
		t.Fatalf("request.go inválido após set: %v\n%s", err, got)
	}
}

func TestRequestValidatorSet_AlreadySet(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := RequestValidatorSet(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"}); err != nil {
		t.Fatalf("first set: %v", err)
	}
	_, err := RequestValidatorSet(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"})
	if err == nil || !strings.Contains(err.Error(), "já configurado") {
		t.Fatalf("second set: want erro 'já configurado', got %v", err)
	}
}

func TestRequestValidatorSet_MissingRequest(t *testing.T) {
	root := t.TempDir()
	_, err := RequestValidatorSet(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"})
	if err == nil {
		t.Fatalf("RequestValidatorSet: esperado erro para request inexistente")
	}
}

func TestRequestValidatorUnset_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := RequestValidatorSet(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	rel, err := RequestValidatorUnset(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"})
	if err != nil {
		t.Fatalf("RequestValidatorUnset: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, "validator services.Validator") {
		t.Errorf("campo validator ainda presente:\n%s", got)
	}
	if strings.Contains(got, "ValidateStruct") {
		t.Errorf("body de Validate ainda referencia ValidateStruct:\n%s", got)
	}
	mustContain(t, got,
		"func NewRequest(data *Data) *Request",
		"return &Request{Data: data}",
		"return nil",
	)
	if strings.Contains(got, `"zord/internal/application/services"`) {
		t.Errorf("import services não foi prunado:\n%s", got)
	}
	if err := parseGoSrc([]byte(got)); err != nil {
		t.Fatalf("request.go inválido após unset: %v\n%s", err, got)
	}
}

func TestRequestValidatorUnset_NotSet(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	_, err := RequestValidatorUnset(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"})
	if err == nil || !strings.Contains(err.Error(), "não configurado") {
		t.Fatalf("unset: want erro 'não configurado', got %v", err)
	}
}

func TestRequestValidatorSet_KeepsExistingDataFields(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Email", TypeStr: "string", Validate: "required,email",
	}); err != nil {
		t.Fatalf("RequestFieldAdd: %v", err)
	}
	if _, err := RequestValidatorSet(RequestValidatorOptions{Root: root, Domain: "Foo", Verb: "Create"}); err != nil {
		t.Fatalf("RequestValidatorSet: %v", err)
	}
	relPath, _ := requestFilePath(root, "Foo", "Create")
	src, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	mustContain(t, string(src),
		"Email string",
		"validate:\"required,email\"",
		"validator services.Validator",
	)
}
