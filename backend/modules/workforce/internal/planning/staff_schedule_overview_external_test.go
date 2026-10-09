package planning

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
)

// TestStaffScheduleOverview_LeavesExternalCaregiversOut pins #3823: an
// external caregiver has no shifts to plan, so the duty roster has no row
// for them.
func TestStaffScheduleOverview_LeavesExternalCaregiversOut(t *testing.T) {
	t.Parallel()

	employee := fakeStaff(1, "Ada", "Lovelace")
	external := fakeStaff(2, "Ella", "Extern")
	external.IsGuest = true
	employee.ID, external.ID = 1, 2
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts:        &fakeShiftReader{},
		Instances:     &fakeInstanceReader{},
		InstanceStaff: &fakeInstanceStaffReader{},
		Rooms:         &fakeRoomReader{},
		Staff:         &fakeStaffReader{rows: []*users.Staff{employee, external}},
	})

	monday := timezone.NewDate(2026, time.July, 6)
	got, err := service.GetOverview(context.Background(), monday, monday.AddDays(4))
	require.NoError(t, err)
	require.Len(t, got.Staff, 1)
	require.Equal(t, employee.ID, got.Staff[0].ID)
}
