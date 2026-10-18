package compose

import (
	"context"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The retained repository adapters, the Care Plan and Workforce rows, the
// cleanup and the legacy repository vocabulary live in internal/application,
// the types the roots name in the ports package (#4084). The roots and the
// suites build them through the constructors below.

// ActivityInstanceDate is the retained instance date of a calendar day.
func ActivityInstanceDate(day calendar.Date) scheduleModels.Date {
	return scheduleModels.Date(day)
}

// ParseActivityInstanceDate parses a retained instance date.
func ParseActivityInstanceDate(value string) (scheduleModels.Date, error) {
	return application.ParseActivityInstanceDate(value)
}

// WrapDatabaseError wraps err as the retained repositories' database error.
func WrapDatabaseError(operation string, err error) error {
	return application.WrapDatabaseError(operation, err)
}

// FindCalendarAssignments reads a staff member's instance assignments.
func FindCalendarAssignments(ctx context.Context, rows ports.CalendarAssignmentRows, staffID int64, from, to string) ([]ports.CalendarAssignment, error) {
	return application.FindCalendarAssignments(ctx, rows, staffID, from, to)
}

// FindCalendarInstances reads activity instances by ID.
func FindCalendarInstances(ctx context.Context, rows ports.CalendarInstanceRows, ids []int64) ([]ports.CalendarInstance, error) {
	return application.FindCalendarInstances(ctx, rows, ids)
}

// FindCalendarShifts reads a staff member's shifts.
func FindCalendarShifts(ctx context.Context, rows ports.CalendarShiftRows, staffID int64, from, to string) ([]ports.CalendarShift, error) {
	return application.FindCalendarShifts(ctx, rows, staffID, from, to)
}

// ListCalendarShiftTypes reads the shift types.
func ListCalendarShiftTypes(ctx context.Context, rows ports.CalendarShiftTypeRows) ([]ports.CalendarShiftType, error) {
	return application.ListCalendarShiftTypes(ctx, rows)
}

// NewActivityCategorySeeder binds the category seeder to the retained
// category rows.
func NewActivityCategorySeeder(categories activitiesModels.CategoryRepository) application.ActivityCategorySeeder {
	return application.NewActivityCategorySeeder(categories)
}

// NewCallerActivityGroups reads the activity groups a caller supervises.
func NewCallerActivityGroups(groups activitiesModels.GroupRepository) ports.CallerActivityGroups {
	return application.NewCallerActivityGroups(groups)
}

// NewCareArrivalScheduleRows serves the arrival schedules from Care Plan.
func NewCareArrivalScheduleRows(rows ports.CarePlanRows) ports.CareArrivalScheduleRows {
	return application.NewCareArrivalScheduleRows(rows)
}

// NewCareArrivalExceptionRows serves the arrival exceptions from Care Plan.
func NewCareArrivalExceptionRows(rows ports.CarePlanRows) ports.CareArrivalExceptionRows {
	return application.NewCareArrivalExceptionRows(rows)
}

// NewCarePickupScheduleRows serves the pickup schedules from Care Plan.
func NewCarePickupScheduleRows(rows ports.CarePlanRows) ports.CarePickupScheduleRows {
	return application.NewCarePickupScheduleRows(rows)
}

// NewCarePickupExceptionRows serves the pickup exceptions from Care Plan.
func NewCarePickupExceptionRows(rows ports.CarePlanRows) ports.CarePickupExceptionRows {
	return application.NewCarePickupExceptionRows(rows)
}

// NewCategoryCount counts the activity categories.
func NewCategoryCount(records application.CategoryCountRecords) *application.CategoryCount {
	return application.NewCategoryCount(records)
}

// NewEndedSessionCompletion completes the blocks of ended sessions.
func NewEndedSessionCompletion(deps ports.EndedSessionCompletionDependencies) (timetable.EndedSessionCompletion, error) {
	return application.NewEndedSessionCompletion(deps)
}

// NewEnrollmentArrivalSchedules binds Enrollment's weekly arrival port.
func NewEnrollmentArrivalSchedules(repo scheduleModels.StudentArrivalScheduleRepository) application.EnrollmentArrivalSchedules {
	return application.NewEnrollmentArrivalSchedules(repo)
}

// NewEnrollmentPickupSchedules binds Enrollment's weekly pickup port.
func NewEnrollmentPickupSchedules(repo scheduleModels.StudentPickupScheduleRepository) application.EnrollmentPickupSchedules {
	return application.NewEnrollmentPickupSchedules(repo)
}

// NewPresenceSessionFacts reads the Student Presence session facts.
func NewPresenceSessionFacts(presence application.PresenceSessions) timetable.SessionFacts {
	return application.NewPresenceSessionFacts(presence)
}

// NewTimetableCleanup builds the retention cleanup.
func NewTimetableCleanup(deps ports.TimetableCleanupDependencies) (timetable.TimetableCleanup, error) {
	return application.NewTimetableCleanup(deps)
}

// NewTimetableGroupReads reads the activity groups.
func NewTimetableGroupReads(groups activitiesModels.GroupRepository) application.TimetableGroupReads {
	return application.NewTimetableGroupReads(groups)
}

// NewTimetableInstanceReads reads the activity instances.
func NewTimetableInstanceReads(instances scheduleModels.ActivityInstanceRepository) application.TimetableInstanceReads {
	return application.NewTimetableInstanceReads(instances)
}

// NewTimetableInstanceStaffReads reads the instance staff.
func NewTimetableInstanceStaffReads(staff scheduleModels.InstanceStaffRepository) application.TimetableInstanceStaffReads {
	return application.NewTimetableInstanceStaffReads(staff)
}

// NewWorkforceShiftRows serves the staff shifts from Workforce.
func NewWorkforceShiftRows(records application.WorkforceShiftRecords, errs ports.RetainedRecordErrors) ports.WorkforceShiftRows {
	return application.NewWorkforceShiftRows(records, errs)
}

// NewWorkforceShiftTypeRows serves the shift types from Workforce.
func NewWorkforceShiftTypeRows(records application.WorkforceShiftRecords, errs ports.RetainedRecordErrors) ports.WorkforceShiftTypeRows {
	return application.NewWorkforceShiftTypeRows(records, errs)
}

// ShiftTypeCategoryLinker links a shift type to its activity categories.
func ShiftTypeCategoryLinker(link func(context.Context, int64, []int64) error) func(context.Context, int64, []int64) error {
	return application.ShiftTypeCategoryLinker(link)
}
