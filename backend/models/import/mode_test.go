package importpkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImportMode(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]ImportMode{"": ImportModeCreate, "create": ImportModeCreate, " Update ": ImportModeUpdate, "UPSERT": ImportModeUpsert} {
		got, err := ParseImportMode(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
	_, err := ParseImportMode("merge")
	require.Error(t, err)
}
