package testdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIdentifierHashIsStableAndSourceSensitive pins the identifier digest so a
// change of algorithm shows up as a deliberate rename of every template and
// clone, not as a silent cache miss.
func TestIdentifierHashIsStableAndSourceSensitive(t *testing.T) {
	t.Parallel()
	got := identifierHash("services/active")
	assert.Equal(t, got, identifierHash("services/active"))
	assert.Len(t, got, 32)
	assert.Regexp(t, `^[0-9a-f]{32}$`, got)
	assert.NotEqual(t, got, identifierHash("services/active/"))
	// FNV-1a 128 of the empty string is its offset basis.
	assert.Equal(t, "6c62272e07bb014262b821756295c58d", identifierHash(""))
}
