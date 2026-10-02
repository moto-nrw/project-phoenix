package timetablehttp

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateResponseIncludesPlanningTrackMetadata(t *testing.T) {
	t.Parallel()

	response := templateResponseFromRow(templateRow{TemplateListRow: timetable.TemplateListRow{
		TemplateID:         41,
		Name:               "Lernzeit",
		Type:               timetable.GroupTypeCare,
		CategoryID:         9,
		CategoryName:       "Lernzeit",
		PlanningTrackID:    testpkg.Int64Ptr(7),
		PlanningTrackName:  "Jahrgang 1",
		PlanningTrackColor: "#5080D8",
		PlanningTrackOrder: testpkg.Int64Ptr(2),
	}}, 10)

	require.NotNil(t, response.PlanningTrackID)
	assert.Equal(t, "Jahrgang 1", response.PlanningTrackName)
	assert.Equal(t, "#5080D8", response.PlanningTrackColor)
	require.NotNil(t, response.PlanningTrackSortOrder)
	assert.EqualValues(t, 2, *response.PlanningTrackSortOrder)
}
