package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/services"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The live-flag write is a narrow column update rather than the owner's
// full-row save. A partial update reports "no rows matched" instead of
// failing, so the adapter has to turn that into an error itself: the check-in
// auto-clear runs inside the visit transaction and must roll the visit back
// when the child it is clearing has vanished or moved tenant.
func TestPresenceStudents_UpdateLiveStatusFailsOnUnknownStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	students := services.PresenceStudents(repoFactory.Student)

	sick := true
	now := time.Now()
	err := students.UpdateLiveStatus(testpkg.Ctx(t), &activeService.StudentRecord{
		ID:        999_000_111,
		TenantID:  testpkg.Tenant(t),
		Sick:      &sick,
		SickSince: &now,
	})

	require.Error(t, err, "a live-status write against a missing student must not pass silently")
	assert.Contains(t, err.Error(), "999000111")
}

// The locked-row re-authorization is the status-day write's access control, so
// an adapter built without one must refuse every write rather than wave them
// through. A caller that genuinely has no identity to check says so with
// services.AllowAllStatusDayWrites.
func TestStatusDayStudents_MissingAuthorizeFailsClosed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	student := testpkg.CreateTestStudent(t, db, "FailClosed", "Student", "FC1")
	ctx := testpkg.Ctx(t)

	unguarded := services.StatusDayStudentsFromRepository(repoFactory.Student, nil)
	_, err := unguarded.LockForStatusWrite(ctx, student.ID, activeModels.StudentStatusDaySick)
	require.Error(t, err, "a status-day adapter without an authorization callback must refuse the write")

	guarded := services.StatusDayStudentsFromRepository(repoFactory.Student, services.AllowAllStatusDayWrites)
	record, err := guarded.LockForStatusWrite(ctx, student.ID, activeModels.StudentStatusDaySick)
	require.NoError(t, err)
	assert.Equal(t, student.ID, record.ID)
}

// The happy path proves the four flag columns actually land, so the guard
// above is not simply rejecting every write.
func TestPresenceStudents_UpdateLiveStatusPersistsFlags(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	students := services.PresenceStudents(repoFactory.Student)
	student := testpkg.CreateTestStudent(t, db, "Live", "Status", "LS1")
	ctx := testpkg.Ctx(t)

	sick := true
	since := time.Now().Truncate(time.Second)
	require.NoError(t, students.UpdateLiveStatus(ctx, &activeService.StudentRecord{
		ID: student.ID, TenantID: testpkg.Tenant(t), Sick: &sick, SickSince: &since,
	}))

	reloaded, err := students.FindByID(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Sick)
	assert.True(t, *reloaded.Sick)
	require.NotNil(t, reloaded.SickSince)
	// The excused column defaults to FALSE and this write does not name it,
	// so the untouched flag comes back false rather than changing with sick.
	require.NotNil(t, reloaded.Excused)
	assert.False(t, *reloaded.Excused)
	assert.Nil(t, reloaded.ExcusedSince)
}
