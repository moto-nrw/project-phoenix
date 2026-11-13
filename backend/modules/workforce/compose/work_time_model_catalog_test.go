package compose

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func buildWorkTime(t *testing.T, db *bun.DB) WorkTime {
	t.Helper()
	runtime := testpkg.ConfigRuntime(db)
	workTime, err := New(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment,
		DB: db, LiveStaffIDs: runtime.LiveStaffIDs, Observe: func(Observation) {}})
	require.NoError(t, err)
	return workTime
}

func catalogModel(t *testing.T, db *bun.DB, workTime WorkTime, name string) workforce.WorkTimeModel {
	t.Helper()
	model, err := workTime.CreateWorkTimeModel(testpkg.Ctx(t), workforce.CreateWorkTimeModel{WorkTimeModelFields: workforce.WorkTimeModelFields{
		Name: name, RotationLength: 1, RotationAnchorDate: "2026-01-05",
		Entries: []workforce.WorkTimeModelEntry{{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 240}},
	}})
	require.NoError(t, err)
	testpkg.OwnTestWorkTimeModel(t, db, model.ID)
	return model
}

// The catalog page shows how many people a template change reaches (#6891):
// the live assignees, the same set a template edit rewrites. A retired
// membership keeps its binding (ADR 0021) but is no longer reached.
func TestWorkTimeModelUsageCountsLiveAssigneesPerTemplate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	workTime := buildWorkTime(t, db)
	employment := buildStaffEmployment(t, db)
	ctx := testpkg.Ctx(t)
	fullTime := catalogModel(t, db, workTime, "Vollzeit")
	partTime := catalogModel(t, db, workTime, "Teilzeit")
	unused := catalogModel(t, db, workTime, "Ungenutzt")

	first := testpkg.CreateTestStaff(t, db, "Usage", "First")
	second := testpkg.CreateTestStaff(t, db, "Usage", "Second")
	retired := testpkg.CreateTestStaff(t, db, "Usage", "Retired")
	part := testpkg.CreateTestStaff(t, db, "Usage", "Part")
	for staffID, modelID := range map[int64]int64{first.ID: fullTime.ID, second.ID: fullTime.ID, retired.ID: fullTime.ID, part.ID: partTime.ID} {
		require.NoError(t, employment.SaveStaffEmployment(ctx, workforce.StaffEmployment{MembershipID: staffID, WorkTimeModelID: &modelID}))
	}
	retireMembership(t, db, retired.ID)

	usage, err := workTime.WorkTimeModelUsage(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{fullTime.ID: 2, partTime.ID: 1}, usage, "an unused template has no entry")
	assert.NotContains(t, usage, unused.ID)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	foreign, err := workTime.WorkTimeModelUsage(testpkg.TenantContext(otherTenantID))
	require.NoError(t, err)
	assert.Empty(t, foreign, "another school sees none of these bindings")
}

// Archiving takes a template out of the choice for new assignments; it keeps
// its rows and its assignees, and restoring offers it again.
func TestArchivingAWorkTimeModelIsReversibleAndTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	workTime := buildWorkTime(t, db)
	ctx := testpkg.Ctx(t)
	model := catalogModel(t, db, workTime, "Archiv")
	require.Nil(t, model.ArchivedAt, "a new template is offered")

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	_, err := workTime.ArchiveWorkTimeModel(testpkg.TenantContext(otherTenantID), model.ID)
	require.ErrorIs(t, err, workforce.ErrWorkTimeModelNotFound, "another school cannot archive it")

	archived, err := workTime.ArchiveWorkTimeModel(ctx, model.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt)
	assert.Len(t, archived.Entries, 1, "archiving keeps the entries")
	again, err := workTime.ArchiveWorkTimeModel(ctx, model.ID)
	require.NoError(t, err)
	assert.Equal(t, archived.ArchivedAt.UTC(), again.ArchivedAt.UTC(), "archiving twice keeps the first instant")

	listed, err := workTime.ListWorkTimeModels(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1, "the list keeps archived templates for the catalog")
	assert.NotNil(t, listed[0].ArchivedAt)

	restored, err := workTime.RestoreWorkTimeModel(ctx, model.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.ArchivedAt)

	_, err = workTime.RestoreWorkTimeModel(ctx, 9_223_372_036_854_775_000)
	require.ErrorIs(t, err, workforce.ErrWorkTimeModelNotFound)
}
