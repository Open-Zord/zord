package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSetGet(t *testing.T) {
	c := NewCache()
	c.Set("k", "v", 0)

	got, ok := c.Get("k")
	assert.True(t, ok)
	assert.Equal(t, "v", got)
}

func TestGetMissing(t *testing.T) {
	c := NewCache()
	got, ok := c.Get("absent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestTTLExpires(t *testing.T) {
	c := NewCache()
	c.Set("k", 123, 20*time.Millisecond)

	got, ok := c.Get("k")
	assert.True(t, ok)
	assert.Equal(t, 123, got)

	time.Sleep(40 * time.Millisecond)

	_, ok = c.Get("k")
	assert.False(t, ok, "entrada deveria ter expirado")
	assert.Equal(t, 0, c.Len(), "entrada expirada deveria ter sido removida no Get")
}

func TestZeroTTLNeverExpires(t *testing.T) {
	c := NewCache()
	c.Set("k", "v", 0)
	time.Sleep(20 * time.Millisecond)

	_, ok := c.Get("k")
	assert.True(t, ok)
}

func TestDelete(t *testing.T) {
	c := NewCache()
	c.Set("k", "v", 0)
	c.Delete("k")

	_, ok := c.Get("k")
	assert.False(t, ok)
	assert.Equal(t, 0, c.Len())
}
