package registry

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	assert.NotNil(t, r)
	assert.NotNil(t, r.deps)
}

func TestProvideAndInject(t *testing.T) {
	r := NewRegistry()

	dep := "sampleDependency"
	r.Provide("sample", dep)

	injectedDep := r.Inject("sample")
	assert.Equal(t, dep, injectedDep)
}

func TestInjectPanicsWhenMissing(t *testing.T) {
	r := NewRegistry()

	defer func() {
		rec := recover()
		assert.NotNil(t, rec, "expected panic")
		msg, ok := rec.(string)
		assert.True(t, ok)
		assert.Contains(t, msg, `"nonexistent"`)
		assert.Contains(t, msg, "not registered")
	}()
	r.Inject("nonexistent")
}

type sampleSvc struct{ name string }

type sampleSvcPort interface {
	Name() string
}

func (s *sampleSvc) Name() string { return s.name }

func TestResolveHappyPath(t *testing.T) {
	r := NewRegistry()
	svc := &sampleSvc{name: "ok"}
	r.Provide("svc", svc)

	got := Resolve[*sampleSvc](r, "svc")
	assert.Equal(t, svc, got)
	assert.Equal(t, "ok", got.Name())
}

func TestResolveViaInterface(t *testing.T) {
	r := NewRegistry()
	r.Provide("svc", &sampleSvc{name: "via-iface"})

	got := Resolve[sampleSvcPort](r, "svc")
	assert.Equal(t, "via-iface", got.Name())
}

func TestResolvePanicsWhenMissing(t *testing.T) {
	r := NewRegistry()

	defer func() {
		rec := recover()
		assert.NotNil(t, rec, "expected panic")
		msg, _ := rec.(string)
		assert.Contains(t, msg, `"missing"`)
	}()
	_ = Resolve[*sampleSvc](r, "missing")
}

func TestResolvePanicsOnWrongType(t *testing.T) {
	r := NewRegistry()
	r.Provide("svc", &sampleSvc{name: "x"})

	defer func() {
		rec := recover()
		assert.NotNil(t, rec, "expected panic")
		msg, _ := rec.(string)
		assert.Contains(t, msg, `"svc"`)
		assert.True(t, strings.Contains(msg, "expected"), "message should mention the expected type: %q", msg)
	}()
	_ = Resolve[string](r, "svc")
}
