package planning

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
	monday := timezone.NewDate(2026, time.July, 6)
	instance := &timetable.ScheduledInstance{
		Date: timezone.Date(monday), Title: "Musik-AG", StartTime: testClock(t, "14:00"), EndTime: testClock(t, "15:00"), Status: timetable.InstanceStatusPlanned,
	}
	instance.ID = 3
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts:        &fakeShiftReader{},
		Instances:     &fakeInstanceReader{rows: []*timetable.ScheduledInstance{instance}},
		InstanceStaff: &fakeInstanceStaffReader{rows: []*timetable.InstanceStaff{{InstanceID: instance.ID, StaffID: external.ID}}},
		Rooms:         &fakeRoomReader{},
		Staff:         &fakeStaffReader{rows: []*users.Staff{employee, external}},
	})

	got, err := service.GetOverview(context.Background(), monday, monday.AddDays(4))
	require.NoError(t, err)
	require.Len(t, got.Staff, 1)
	require.Equal(t, employee.ID, got.Staff[0].ID)
	require.Empty(t, got.Assignments, "external caregivers have no Dienstplan assignments")
}

func TestStaffScheduleOverview_KeepsAccountBackedGuestStaff(t *testing.T) {
	t.Parallel()

	employee := fakeStaff(1, "Ada", "Lovelace")
	guest := fakeStaff(2, "Grace", "Hopper")
	accountID := int64(42)
	guest.IsGuest = true
	guest.Person.AccountID = &accountID
	employee.ID, guest.ID = 1, 2
	monday := timezone.NewDate(2026, time.July, 6)
	instance := &timetable.ScheduledInstance{
		Date: timezone.Date(monday), Title: "Robotik-AG", StartTime: testClock(t, "14:00"), EndTime: testClock(t, "15:00"), Status: timetable.InstanceStatusPlanned,
	}
	instance.ID = 3
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts:        &fakeShiftReader{},
		Instances:     &fakeInstanceReader{rows: []*timetable.ScheduledInstance{instance}},
		InstanceStaff: &fakeInstanceStaffReader{rows: []*timetable.InstanceStaff{{InstanceID: instance.ID, StaffID: guest.ID}}},
		Rooms:         &fakeRoomReader{},
		Staff:         &fakeStaffReader{rows: []*users.Staff{employee, guest}},
	})

	got, err := service.GetOverview(context.Background(), monday, monday.AddDays(4))
	require.NoError(t, err)
	require.Len(t, got.Staff, 2)
	require.Len(t, got.Assignments, 1)
	require.Equal(t, guest.ID, got.Assignments[0].StaffID)
}
