package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Die Reihe gehört seit #2180 zur gespeicherten Anordnung: eine Kachel, die
// zwischen zwei Reihen abgelegt wird, bekommt eine eigene. Der Server prüft
// nur die Form, nicht die Bedeutung.
func TestValidateHomeBlockPlacements_Rows(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Row: 0},
		{Key: "section.open_requests", Span: 2, Row: 1},
		{Key: "section.staff_today", Span: 2, Row: 1},
	}), "Reihen dürfen sich wiederholen und Lücken haben; sortiert wird im Frontend")

	err := ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Row: -1},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid row")

	err = ValidateHomeBlockPlacements([]HomeBlockPlacement{
		{Key: "tile.students_present", Span: 1, Row: MaxHomeBlockEntries},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid row")
}
