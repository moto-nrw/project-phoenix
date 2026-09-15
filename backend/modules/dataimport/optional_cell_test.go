package dataimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// stringPtr Tests
// ============================================================================

func TestStringPtr(t *testing.T) {
	t.Parallel()
	t.Run("returns pointer to string", func(t *testing.T) {
		result := OptionalCell("test")
		assert.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		result := OptionalCell("")
		assert.Nil(t, result)
	})

	t.Run("returns nil for whitespace-only string", func(t *testing.T) {
		result := OptionalCell("   ")
		assert.Nil(t, result)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		result := OptionalCell("  test  ")
		assert.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})
}
