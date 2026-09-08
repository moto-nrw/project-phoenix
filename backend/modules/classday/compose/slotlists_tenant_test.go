package compose_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestBuildList_TenantIsolation proves the projection's tenant safety (#2701):
// a second school's slot and children on the same date never reach the first
// school's list, and the second school sees only its own rows, even though
// every owner facade reads the same tables.
func TestBuildList_TenantIsolation(t *testing.T) {
	t.Parallel()
	f := buildMensaFixture(t)
	ctx := testpkg.Ctx(t)

	otherTenant, _ := testpkg.CreateTestTenant(t, f.db)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoomForTenant(t, f.db, otherTenant, fmt.Sprintf("Other-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroupForTenant(t, f.db, otherTenant, fmt.Sprintf("Other-Act-%d", suffix))
	activeGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, f.db, otherTenant, activity.ID, room.ID)
	instance := testpkg.CreateTestActivityInstanceForTenant(t, f.db, otherTenant, listDate, room.ID, testpkg.ActivityInstanceOpts{
		Title: fmt.Sprintf("Other Mensa %d", suffix), Status: "active", ActivityGroupID: &activity.ID, ActiveGroupID: &activeGroup.ID,
		StartHHMM: "11:30", EndHHMM: "13:00",
	})
	otherStudent := testpkg.CreateTestStudentForTenant(t, f.db, otherTenant, "Other", fmt.Sprintf("Kind-%d", suffix), "2b")

	params := classday.Params{Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourceReconciliation}

	own, err := f.svc.BuildList(ctx, params)
	require.NoError(t, err)
	for _, slot := range own.Slots {
		assert.NotEqual(t, instance.ID, slot.InstanceID, "the other school's slot must not be listed")
	}
	assert.Nil(t, rowByStudent(own.Rows, otherStudent.ID), "the other school's child must not be listed")
	require.NotNil(t, rowByStudent(own.Rows, f.plannedID))

	other, err := f.svc.BuildList(testpkg.TenantContext(otherTenant), params)
	require.NoError(t, err)
	require.Len(t, other.Slots, 1)
	assert.Equal(t, instance.ID, other.Slots[0].InstanceID)
	assert.Empty(t, other.Rows, "the other school has no roster and no visits on its slot")
	assert.Nil(t, rowByStudent(other.Rows, f.plannedID))
}
