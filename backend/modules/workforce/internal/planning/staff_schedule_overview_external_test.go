package planning

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

// TestStaffScheduleOverview_LeavesExternalCaregiversOut pins #3823: an
// external caregiver has no shifts to plan, so the duty roster has no row
// for them.
func TestStaffScheduleOverview_LeavesExternalCaregiversOut(t *testing.T) {
	t.Parallel()

	employee := fakeStaff(1, "Ada", "Lovelace")
	external := fakeStaff(2, "Ella", "Extern")
	external.IsExternalCaregiver = true
	employee.ID, external.ID = 1, 2
	monday := calendar.NewDate(2026, time.July, 6)
	instance := &timetable.ScheduledInstance{
		Date: calendar.Date(monday), Title: "Musik-AG", StartTime: testClock(t, "14:00"), EndTime: testClock(t, "15:00"), Status: timetable.InstanceStatusPlanned,
	}
	instance.ID = 3
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts:        &fakeShiftReader{},
		Instances:     &fakeInstanceReader{rows: []*timetable.ScheduledInstance{instance}},
		InstanceStaff: &fakeInstanceStaffReader{rows: []*timetable.InstanceStaff{{InstanceID: instance.ID, StaffID: external.ID}}},
		Rooms:         &fakeRoomReader{},
		Staff:         &fakeStaffReader{rows: []*workforce.PlanStaff{employee, external}},
	})

	got, err := service.GetOverview(context.Background(), monday, monday.AddDays(4))
	require.NoError(t, err)
	require.Len(t, got.Staff, 1)
	require.Equal(t, employee.ID, got.Staff[0].ID)
	require.Empty(t, got.Assignments, "external caregivers have no Dienstplan assignments")
}

// TestStaffScheduleOverview_LeavesAccountBackedExternalCaregiversOut pins
// #4573: an account does not make an external caregiver Dienstplan staff.
// Before #4573 an account-backed external caregiver kept their row and assignments.
func TestStaffScheduleOverview_LeavesAccountBackedExternalCaregiversOut(t *testing.T) {
	t.Parallel()

	employee := fakeStaff(1, "Ada", "Lovelace")
	external := fakeStaff(2, "Grace", "Hopper")
	accountID := int64(42)
	external.IsExternalCaregiver = true
	external.Person.AccountID = &accountID
	employee.ID, external.ID = 1, 2
	monday := calendar.NewDate(2026, time.July, 6)
	instance := &timetable.ScheduledInstance{
		Date: calendar.Date(monday), Title: "Robotik-AG", StartTime: testClock(t, "14:00"), EndTime: testClock(t, "15:00"), Status: timetable.InstanceStatusPlanned,
	}
	instance.ID = 3
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts:    &fakeShiftReader{},
		Instances: &fakeInstanceReader{rows: []*timetable.ScheduledInstance{instance}},
		InstanceStaff: &fakeInstanceStaffReader{rows: []*timetable.InstanceStaff{
			{InstanceID: instance.ID, StaffID: external.ID}, {InstanceID: instance.ID, StaffID: employee.ID},
		}},
		Rooms: &fakeRoomReader{},
		Staff: &fakeStaffReader{rows: []*workforce.PlanStaff{employee, external}},
	})

	got, err := service.GetOverview(context.Background(), monday, monday.AddDays(4))
	require.NoError(t, err)
	require.Len(t, got.Staff, 1)
	require.Equal(t, employee.ID, got.Staff[0].ID)
	require.Len(t, got.Assignments, 1, "the employee keeps the assignment, the external has none")
	require.Equal(t, employee.ID, got.Assignments[0].StaffID)
}
