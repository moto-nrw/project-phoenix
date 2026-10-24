package application

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingPickupStaff struct{ tenants []int64 }

func (s *recordingPickupStaff) PickupScheduleChanged(tenantID int64) error {
	s.tenants = append(s.tenants, tenantID)
	return nil
}

type recordingPickupGuardians struct{ calls [][2]int64 }

func (g *recordingPickupGuardians) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	g.calls = append(g.calls, [2]int64{tenantID, studentID})
}

// A changed offering pickup projection (#3560) wakes the staff pickup views
// of the school and every affected child's guardians, but only once the
// transaction committed.
func TestAnnounceOfferingPickupChangeWakesStaffAndGuardiansAfterCommit(t *testing.T) {
	t.Parallel()
	staff, guardians, hooks := &recordingPickupStaff{}, &recordingPickupGuardians{}, newFakeHooks()
	announce := AnnounceOfferingPickupChange(staff, guardians, hooks, slog.Default())
	require.NotNil(t, announce)

	announce(context.Background(), []int64{fakeStudentID, otherStudentID})
	assert.Empty(t, staff.tenants, "a rolled-back change stays silent")
	hooks.commit()

	assert.Equal(t, []int64{fakeTenantID}, staff.tenants)
	assert.Equal(t, [][2]int64{{fakeTenantID, fakeStudentID}, {fakeTenantID, otherStudentID}}, guardians.calls)

	announce(context.Background(), nil)
	hooks.commit()
	assert.Len(t, staff.tenants, 1, "no student, no announcement")
}

func TestAnnounceOfferingPickupChangeNothingToWake(t *testing.T) {
	t.Parallel()
	assert.Nil(t, AnnounceOfferingPickupChange(nil, nil, newFakeHooks(), slog.Default()),
		"without staff views and guardians there is nobody to announce to")
	only, hooks := &recordingPickupGuardians{}, newFakeHooks()
	announce := AnnounceOfferingPickupChange(nil, only, hooks, nil)
	require.NotNil(t, announce)
	announce(context.Background(), []int64{fakeStudentID})
	hooks.commit()
	assert.Equal(t, [][2]int64{{fakeTenantID, fakeStudentID}}, only.calls, "the guardians hear about it without staff views")
}
