package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spalte und Zeile gehören seit #2180 zur gespeicherten Anordnung: das Brett
// ist ein freies Raster, jede Kachel liegt in ihrer Zelle. Der Server prüft
// nur die Form, nicht die Bedeutung: Überlappungen löst das Frontend auf.
func TestValidateHomeBlockPlacements_Cells(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Col: 3, Row: 0},
		{Key: "section.open_requests", Span: 2, Col: 0, Row: 4},
		{Key: "section.staff_today", Span: 2, Col: 2, Row: 4},
		{Key: "section.birthdays", Span: 4, Col: 0, Row: 9},
	}), "Lücken und Zeilen ohne Kachel sind erlaubt")

	err := ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Col: -1, Row: 0},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid column")

	err = ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "section.open_requests", Span: 2, Col: 3, Row: 0},
	})
	require.Error(t, err, "eine breite Kachel darf nicht über den rechten Rand ragen")
	assert.Contains(t, err.Error(), "invalid column")

	err = ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Col: int(^uint(0) >> 1), Row: 0},
	})
	require.Error(t, err, "eine überlaufende Spalte darf nicht als gültig gelten")
	assert.Contains(t, err.Error(), "invalid column")

	err = ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Col: 0, Row: -1},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid row")

	err = ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Col: 0, Row: MaxHomeBlockEntries * 2},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid row")
}
