package students

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/listexport"
)

func healthListStudents() []StudentResponse {
	return []StudentResponse{
		{ID: 1, FirstName: "Mila", LastName: "Anders", HealthInfo: "Nussallergie"},
		{ID: 2, FirstName: "Jonas", LastName: "Ernst"},
		{ID: 3, FirstName: "Lea", LastName: "Brandt", HealthInfo: "   "},
	}
}

func responseIDs(students []StudentResponse) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	return ids
}

// By default the Gesundheitsliste carries only children with a stored note; a
// whitespace-only note counts as none.
func TestHealthListFilterKeepsOnlyChildrenWithNote(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(healthListStudents(), studentExportFilters{}, listexport.PresetHealthList, testExportDate)

	assert.Equal(t, []int64{1}, responseIDs(got))
}

func TestHealthListFilterIncludesChildrenWithoutNoteOnRequest(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(healthListStudents(),
		studentExportFilters{IncludeWithoutHealthInfo: true}, listexport.PresetHealthList, testExportDate)

	assert.Equal(t, []int64{1, 2, 3}, responseIDs(got))
}

// The health scope belongs to the Gesundheitsliste alone: another list must
// neither shrink to children with a note nor reveal who has one.
func TestOtherPresetsIgnoreHealthScope(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(healthListStudents(), studentExportFilters{}, listexport.PresetBlankChecklist, testExportDate)

	assert.Equal(t, []int64{1, 2, 3}, responseIDs(got))
}

func TestHealthInfoExportCell(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Nicht hinterlegt", healthInfoExportCell(""))
	assert.Equal(t, "Nicht hinterlegt", healthInfoExportCell(" \n "))
	assert.Equal(t, "Nussallergie", healthInfoExportCell("Nussallergie"))
	assert.Equal(t, "Nussallergie", healthInfoExportCell("\x01Nuss\x02allergie"),
		"renderer style markers in parent-written text must not reach the document")
}

func TestAddHealthInfoCells(t *testing.T) {
	t.Parallel()

	students := healthListStudents()
	sources := responseRowSources(students, map[int64]weeklySchedule{}, map[int64]string{}, testExportDate, true)

	require.NoError(t, addHealthInfoCells(sources, students))

	assert.Equal(t, "Nussallergie", sources[0].row.Values[listexport.ColumnHealthInfo])
	assert.Equal(t, "Nicht hinterlegt", sources[1].row.Values[listexport.ColumnHealthInfo])
	assert.Equal(t, "Nicht hinterlegt", sources[2].row.Values[listexport.ColumnHealthInfo])
}

// A row set that no longer lines up with the children is refused rather than
// printing a note next to the wrong name.
func TestAddHealthInfoCellsRefusesMisalignedRows(t *testing.T) {
	t.Parallel()

	students := healthListStudents()
	sources := responseRowSources(students[:2], map[int64]weeklySchedule{}, map[int64]string{}, testExportDate, true)

	require.Error(t, addHealthInfoCells(sources, students))
}

func TestHealthListDocumentFilterLabels(t *testing.T) {
	t.Parallel()

	withNote := exportDocumentFilterLabels(studentExportRequest{Preset: listexport.PresetHealthList}, testExportDate, true)
	assert.Contains(t, withNote, healthListOnlyWithNoteLabel)

	everyone := exportDocumentFilterLabels(studentExportRequest{
		Preset:  listexport.PresetHealthList,
		Filters: studentExportFilters{IncludeWithoutHealthInfo: true},
	}, testExportDate, true)
	assert.NotContains(t, everyone, healthListOnlyWithNoteLabel)

	other := exportDocumentFilterLabels(studentExportRequest{Preset: listexport.PresetOGSWeekly}, testExportDate, true)
	assert.NotContains(t, other, healthListOnlyWithNoteLabel)
}

func TestExportTitleHealthList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Gesundheitsliste", exportTitle(studentExportRequest{Preset: listexport.PresetHealthList}))
}
