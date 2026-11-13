package careplanhttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

func TestNormalizeSickReason(t *testing.T) {
	t.Parallel()

	assert.Nil(t, careplanhttp.TrimPtrToNilForTest(nil), "nil stays nil")

	blank := "   "
	assert.Nil(t, careplanhttp.TrimPtrToNilForTest(&blank), "blank trims to nil")

	raw := "  Fieber, beim Arzt  "
	got := careplanhttp.TrimPtrToNilForTest(&raw)
	require.NotNil(t, got)
	assert.Equal(t, "Fieber, beim Arzt", *got, "non-blank is trimmed")
}
