package repositories

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type activityBookingDirectory struct{ capability timetable.Command }

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
	targets, ok := f.ActivityGroup.(activityGroupTargets)
	if !ok {
		panic("repository factory: activity group repository must serve group targets")
	}
	f.ActivityGroup = timetableActivityGroupRepository{
		activityGroupTargets: targets,
		timetable:            capability,
		groups:               f.schoolStructure,
	}
	f.ActivitySchedule = timetableActivityScheduleRepository{timetable: capability}
	var supervisors activitiesModels.SupervisorPlannedRepository = timetableActivitySupervisorRepository{timetable: capability}
	if f.schoolMembership != nil {
		supervisors = staffSupervisorPlannedRepository{SupervisorPlannedRepository: supervisors, membership: f.schoolMembership}
	}
	if f.peopleDirectoryBound {
		supervisors = personSupervisorPlannedRepository{SupervisorPlannedRepository: supervisors, persons: f.students}
	}
	f.ActivitySupervisor = supervisors
	enrollments := activitiesModels.StudentEnrollmentRepository(timetableStudentEnrollmentRepository{timetable: capability, students: f.students})
	if f.peopleDirectoryBound {
		enrollments = personStudentEnrollmentRepository{StudentEnrollmentRepository: enrollments, persons: f.students}
	}
	f.StudentEnrollment = enrollments
	f.Timeframe = timetableTimeframeRepository{timetable: capability}
}
