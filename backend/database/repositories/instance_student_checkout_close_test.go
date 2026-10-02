package repositories_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Worker's daily session end closes the visits in bulk and then stamps
// the checkout on the slot attendance of the ended sessions through this
// repository (#2746): the observed presence stays, the open checkout closes,
// a child who never checked in is left to the Timetable completion.
func TestCloseOpenCheckoutsByActiveGroupIDsStampsTheOpenCheckout(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Checkout Close Room %d", time.Now().UnixNano()))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Checkout Close Activity %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Status:        "active",
		ActiveGroupID: &activeGroup.ID,
		Title:         fmt.Sprintf("Checkout Close Instance %d", time.Now().UnixNano()),
		IsSpontaneous: true,
	})
	expectedStudent := testpkg.CreateTestStudent(t, db, "CheckoutClose", "Expected", "9z")
	expectedRow := testpkg.CreateTestInstanceStudent(t, db, instance.ID, expectedStudent.ID, "expected")
	presentStudent := testpkg.CreateTestStudent(t, db, "CheckoutClose", "Present", "9z")
	presentRow := testpkg.CreateTestInstanceStudent(t, db, instance.ID, presentStudent.ID, "present")
	checkedInAt := time.Now().Add(-2 * time.Hour)
	testpkg.UpdateSessionAttendance(t, ctx, db, presentRow.ID, map[string]any{"checked_in_at": checkedInAt})
	repos, err := repositories.NewTimetableTestRepositories(db)
	require.NoError(t, err)

	closed, err := repos.InstanceStudent.CloseOpenCheckoutsByActiveGroupIDs(ctx, []int64{activeGroup.ID}, time.Now())

	require.NoError(t, err)
	assert.Equal(t, 1, closed)
	reloadedPresent := testpkg.InstanceStudentByIDContext(t, ctx, db, presentRow.ID)
	assert.Equal(t, "present", reloadedPresent.Status, "observed presence must be preserved")
	require.NotNil(t, reloadedPresent.CheckedOutAt, "the open slot checkout must close")
	assert.False(t, reloadedPresent.CheckedOutAt.Before(checkedInAt))
	reloadedExpected := testpkg.InstanceStudentByIDContext(t, ctx, db, expectedRow.ID)
	assert.Equal(t, "expected", reloadedExpected.Status)
	assert.Nil(t, reloadedExpected.CheckedOutAt, "a child who never checked in gets no checkout")
}
