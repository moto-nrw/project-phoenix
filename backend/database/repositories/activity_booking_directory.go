package repositories

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type activityBookingDirectory struct{ capability timetable.Command }

func (d activityBookingDirectory) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) error {
	return d.capability.LockPlannedRosterForCareExit(ctx, studentIDs, after)
}

func (d activityBookingDirectory) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]usersRepo.CareExitRemoval, error) {
	rows, err := d.capability.RemovePlannedRosterForCareExit(ctx, studentIDs, after)
	if err != nil {
		return nil, err
	}
	result := make([]usersRepo.CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		result = append(result, usersRepo.CareExitRemoval{
			TenantID: row.TenantID, StudentID: row.StudentID, Kind: usersRepo.CareExitRemovalRoster,
			InstanceID: &row.InstanceID, RoomID: row.RoomID, Status: &row.Status,
			Substatus: row.Substatus, Note: row.Note, IsUnplanned: &row.IsUnplanned,
			NotScheduled: &row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		})
	}
	return result, nil
}

func (d activityBookingDirectory) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, removals []usersRepo.CareExitRemoval) (int, error) {
	rows := make([]timetable.CareExitRosterRow, 0, len(removals))
	for _, removal := range removals {
		if removal.Kind != usersRepo.CareExitRemovalRoster || removal.InstanceID == nil {
			continue
		}
		row := timetable.CareExitRosterRow{
			TenantID: removal.TenantID, StudentID: removal.StudentID, InstanceID: *removal.InstanceID,
			RoomID: removal.RoomID, Substatus: removal.Substatus, Note: removal.Note,
			ManualStatusAt: removal.ManualStatusAt, StudentStatusDayID: removal.StudentStatusDayID,
			PickupExceptionID: removal.PickupExceptionID,
		}
		if removal.Status != nil {
			row.Status = *removal.Status
		}
		if removal.IsUnplanned != nil {
			row.IsUnplanned = *removal.IsUnplanned
		}
		if removal.NotScheduled != nil {
			row.NotScheduled = *removal.NotScheduled
		}
		rows = append(rows, row)
	}
	return d.capability.RestoreRosterForCareExit(ctx, studentIDs, rows)
}

func (d activityBookingDirectory) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) error {
	return d.capability.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
}

func (d activityBookingDirectory) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (usersRepo.ActivityBookingChanges, error) {
	changes, err := d.capability.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
	if err != nil {
		return usersRepo.ActivityBookingChanges{}, err
	}
	result := usersRepo.ActivityBookingChanges{
		Deleted: make([]usersRepo.ActivityBooking, 0, len(changes.Deleted)),
		Capped:  make([]usersRepo.ActivityBookingCap, 0, len(changes.Capped)),
	}
	for _, value := range changes.Deleted {
		result.Deleted = append(result.Deleted, activityBooking(value))
	}
	for _, value := range changes.Capped {
		result.Capped = append(result.Capped, usersRepo.ActivityBookingCap{
			StudentID: value.StudentID, ID: value.ID, PreviousValidUntil: value.PreviousValidUntil,
		})
	}
	return result, nil
}

func (d activityBookingDirectory) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []usersRepo.ActivityBookingRemoval) (int, error) {
	values := make([]timetable.CareExitEnrollmentRemoval, 0, len(removals))
	for _, removal := range removals {
		values = append(values, timetable.CareExitEnrollmentRemoval{
			CareExitEnrollment: publicActivityBooking(removal.ActivityBooking),
			WasDeleted:         removal.WasDeleted, PreviousValidUntil: removal.PreviousValidUntil,
		})
	}
	return d.capability.RestoreStudentEnrollmentsForCareExit(ctx, studentIDs, periodIDs, values)
}

func activityBooking(value timetable.CareExitEnrollment) usersRepo.ActivityBooking {
	return usersRepo.ActivityBooking{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID,
		ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
	}
}

func publicActivityBooking(value usersRepo.ActivityBooking) timetable.CareExitEnrollment {
	return timetable.CareExitEnrollment{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID,
		ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
	}
}

// BindTimetable routes the remaining care-exit enrollment mutations through
// their owner before the service graph captures the repositories.
func (f *Factory) BindTimetable(capability timetable.Capability) {
	if capability == nil {
		panic("repository factory: timetable capability is required")
	}
	repository, ok := f.CareExitCleanup.(*usersRepo.CareExitCleanupRepository)
	if !ok {
		panic("repository factory: care exit cleanup adapter is unavailable")
	}
	repository.BindActivityBookings(activityBookingDirectory{capability: capability})
}
