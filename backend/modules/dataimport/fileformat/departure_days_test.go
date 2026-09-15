package fileformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDepartureDayColumns(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no Gehweise column has a value", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"gehweise.mo": 0, "gehweise.di": 1},
			[]string{"", ""},
		)
		days, err := parseDepartureDayColumns(mapper)
		require.NoError(t, err)
		assert.Nil(t, days)
	})

	t.Run("maps German and canonical values to modes", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"gehweise.mo": 0, "gehweise.di": 1, "gehweise.mi": 2, "gehweise.do": 3},
			[]string{"alleine", "Fährt Bus", "wird abgeholt", "pickup"},
		)
		days, err := parseDepartureDayColumns(mapper)
		require.NoError(t, err)
		require.NotNil(t, days)
		assert.Equal(t, "alone", days["mon"])
		assert.Equal(t, "bus", days["tue"])
		assert.Equal(t, "pickup", days["wed"])
		assert.Equal(t, "pickup", days["thu"])
	})

	t.Run("maps the accompanied mode (Mit anderem Kind) values", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"gehweise.mo": 0, "gehweise.di": 1, "gehweise.mi": 2},
			[]string{"Mit anderem Kind", "begleitet", "accompanied"},
		)
		days, err := parseDepartureDayColumns(mapper)
		require.NoError(t, err)
		require.NotNil(t, days)
		assert.Equal(t, string("accompanied"), days["mon"])
		assert.Equal(t, string("accompanied"), days["tue"])
		assert.Equal(t, string("accompanied"), days["wed"])
	})

	t.Run("rejects unrecognized cell values", func(t *testing.T) {
		mapper := NewColumnMapper(map[string]int{"gehweise.mo": 0}, []string{"taxi"})
		days, err := parseDepartureDayColumns(mapper)
		require.Error(t, err)
		assert.Nil(t, days)
		assert.Contains(t, err.Error(), "gehweise.mo")
		assert.Contains(t, err.Error(), "taxi")
	})

	t.Run("MapStudentRow returns invalid Gehweise value as row error", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "klasse": 2, "gehweise.di": 3},
			[]string{"Max", "Muster", "1a", "buss"},
		)
		_, err := MapStudentRow(mapper)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gehweise.di")
		assert.Contains(t, err.Error(), "buss")
	})

	t.Run("MapStudentRow reads the Begleitung companion note (#1694)", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "klasse": 2, "gehweise.mo": 3, "begleitung": 4},
			[]string{"Max", "Muster", "1a", "mit anderem Kind", "Geschwisterkind Lena"},
		)
		row, err := MapStudentRow(mapper)
		require.NoError(t, err)
		assert.Equal(t, "Geschwisterkind Lena", row.DepartureCompanionNote)
	})
}
