package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
)

func exportColumnIDs(columns []classday.ExportColumn) []classday.ExportColumnID {
	ids := make([]classday.ExportColumnID, 0, len(columns))
	for _, column := range columns {
		ids = append(ids, column.ID)
	}
	return ids
}

// TestSlotListExportColumns_ChecklistOnlyOnPlannedLists guards #2077: a
// planned list prints an empty "Abhaken" column as its last column; actual
// and reconciliation lists already record who came and print none.
func TestSlotListExportColumns_ChecklistOnlyOnPlannedLists(t *testing.T) {
	t.Parallel()

	t.Run("planned list ends with the checklist column", func(t *testing.T) {
		columns := slotListExportColumns(false, true)
		require.NotEmpty(t, columns)
		last := columns[len(columns)-1]
		assert.Equal(t, classday.ExportColumnChecklist, last.ID)
		assert.Equal(t, "Abhaken", last.Label)
	})

	t.Run("planned pickup list keeps the pickup column before the checklist", func(t *testing.T) {
		assert.Equal(t, []classday.ExportColumnID{
			classday.ExportColumnName,
			classday.ExportColumnSchoolClass,
			classday.ExportColumnGroup,
			classday.ExportColumnSlot,
			classday.ExportColumnPlannedPickup,
			classday.ExportColumnPresenceStatus,
			classday.ExportColumnChecklist,
		}, exportColumnIDs(slotListExportColumns(true, true)))
	})

	t.Run("list without checklist prints no checklist column", func(t *testing.T) {
		assert.NotContains(t, exportColumnIDs(slotListExportColumns(false, false)), classday.ExportColumnChecklist)
		assert.NotContains(t, exportColumnIDs(slotListExportColumns(true, false)), classday.ExportColumnChecklist)
	})
}

// TestSlotListExportRows_ChecklistCellStaysEmpty guards #2077: every value
// row of a planned list carries an empty checklist cell to tick by hand, and
// section marker rows stay without values.
func TestSlotListExportRows_ChecklistCellStaysEmpty(t *testing.T) {
	t.Parallel()

	rows := []classday.Row{
		{Name: "Mia Beispiel", SchoolClass: "2a", GroupTitle: "Randstunde"},
		{Name: "Ben Beispiel", SchoolClass: "2b", GroupTitle: "Randstunde"},
	}

	planned := slotListExportRows(rows, false, true)
	require.Len(t, planned, 3)
	assert.Equal(t, "Randstunde", planned[0].GroupTitle)
	assert.Nil(t, planned[0].Values)
	for _, row := range planned[1:] {
		value, ok := row.Values[classday.ExportColumnChecklist]
		assert.True(t, ok, "planned row must carry the checklist cell")
		assert.Empty(t, value)
	}

	actual := slotListExportRows(rows, false, false)
	for _, row := range actual[1:] {
		assert.NotContains(t, row.Values, classday.ExportColumnChecklist)
	}
}
