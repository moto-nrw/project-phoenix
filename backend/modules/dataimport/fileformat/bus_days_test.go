package fileformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBusDayColumns(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no per-day column is present", func(t *testing.T) {
		mapper := NewColumnMapper(map[string]int{"bus": 0}, []string{"Ja"})
		assert.Nil(t, parseBusDayColumns(mapper))
	})

	t.Run("returns nil when per-day headers are present but all cells are blank", func(t *testing.T) {
		// The generated template always emits the Bus.Mo–Bus.Fr headers, so
		// header presence alone must not count as an override (#1580 review).
		mapping := map[string]int{
			"bus": 0, "bus.mo": 1, "bus.di": 2, "bus.mi": 3, "bus.do": 4, "bus.fr": 5,
		}
		values := []string{"Ja", "", "", "", "", ""}
		mapper := NewColumnMapper(mapping, values)
		assert.Nil(t, parseBusDayColumns(mapper))
	})

	t.Run("maps present per-day columns to canonical weekday keys", func(t *testing.T) {
		mapping := map[string]int{"bus.mo": 0, "bus.di": 1, "bus.fr": 2}
		values := []string{"Ja", "Nein", "Ja"}
		mapper := NewColumnMapper(mapping, values)

		days := parseBusDayColumns(mapper)

		require.NotNil(t, days)
		assert.True(t, days["mon"])
		assert.False(t, days["tue"])
		assert.True(t, days["fri"])
		// A column that is not present is absent from the map (not false).
		_, hasWed := days["wed"]
		assert.False(t, hasWed)
	})
}
