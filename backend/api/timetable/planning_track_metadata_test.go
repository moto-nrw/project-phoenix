package timetable

import (
	"testing"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateResponseIncludesPlanningTrackMetadata(t *testing.T) {
	t.Parallel()

	response := templateResponseFromRow(templateRow{
		TemplateID:         41,
		Name:               "Lernzeit",
		Type:               activitiesModel.GroupTypeCare,
		CategoryID:         9,
		CategoryName:       "Lernzeit",
		PlanningTrackID:    activitiesModel.NullInt64{Int64: 7, Valid: true},
		PlanningTrackName:  "Jahrgang 1",
		PlanningTrackColor: "#5080D8",
		PlanningTrackOrder: activitiesModel.NullInt64{Int64: 2, Valid: true},
	}, 10)

	require.NotNil(t, response.PlanningTrackID)
	assert.Equal(t, "Jahrgang 1", response.PlanningTrackName)
	assert.Equal(t, "#5080D8", response.PlanningTrackColor)
	require.NotNil(t, response.PlanningTrackSortOrder)
	assert.EqualValues(t, 2, *response.PlanningTrackSortOrder)
}

func TestInstanceMetadataResolvesPlanningTrackThroughTemplate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Track metadata")
	repos := mustTimetableTestRepositories(db)
	track := &scheduleModel.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 3}
	require.NoError(t, repos.PlanningTrack.Create(scope.Context(), track))
	_, err := db.NewUpdate().Table("activities.groups").
		Set("planning_track_id = ?", track.ID).
		Where("tenant_id = ?", scope.TenantID).
		Where("id = ?", group.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	resource := NewResource(Dependencies{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityGroupRepo: repos.ActivityGroup,
		}),
		PlanningTrackService: scheduleSvc.NewPlanningTrackService(repos.PlanningTrack, db),
	})
	meta := resource.lookupTemplateMeta(
		scope.Context(),
		&group.ID,
		make(map[int64]templateMeta),
		make(map[int64]*scheduleModel.PlanningTrack),
	)

	require.NotNil(t, meta.planningTrackID)
	assert.Equal(t, track.ID, *meta.planningTrackID)
	assert.Equal(t, track.Name, meta.planningTrackName)
	assert.Equal(t, track.Color, meta.planningTrackColor)
	require.NotNil(t, meta.planningTrackSortOrder)
	assert.Equal(t, track.SortOrder, *meta.planningTrackSortOrder)
}
