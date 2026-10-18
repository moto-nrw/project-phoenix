package careplanhttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSickReason(t *testing.T) {
	t.Parallel()

	assert.Nil(t, trimPtrToNil(nil), "nil stays nil")

	blank := "   "
	assert.Nil(t, trimPtrToNil(&blank), "blank trims to nil")

	raw := "  Fieber, beim Arzt  "
	got := trimPtrToNil(&raw)
	require.NotNil(t, got)
	assert.Equal(t, "Fieber, beim Arzt", *got, "non-blank is trimmed")
}
