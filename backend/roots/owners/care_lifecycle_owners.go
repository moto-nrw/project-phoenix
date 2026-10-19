package owners

import (
	"context"
	"fmt"
	"time"

	owner0 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
)

// NewCareLifecycleOwners binds the Care Plan lifecycle's (#3427) ports to the
// owners it drives around a care exit: the child rows and memberships, the
// persons and bracelets, the timetable, the school calendar, the unlogged
// Student Presence, the Timetable plan reads joined with its attendance and
// the Enrollment applications with their bookings. The change trail arrives
// from the caller, which holds the audit service.
func NewCareLifecycleOwners(
	students CareStudentRows, persons CarePersonRows, membership CareMembershipCommands, people peopledirectory.Capability,
	timetables timetable.Capability, calendar schoolcalendar.Query, presence *studentpresence.Module,
	planReads presenceProjectionReads, enrollment EnrollmentBookingProjection, changeTrail carePlanCompose.CareEndRecorder,
) carePlanCompose.CareLifecycleOwners {
	return carePlanCompose.CareLifecycleOwners{
		Students: CareStudents{
			students: students, persons: persons,
			membership: membership, people: people,
		},
		Roster:      careexit.NewRoster(timetables, presence, planReads),
		Bookings:    careExitBookings{capability: timetables},
		Enrollment:  careExitEnrollment{projection: enrollment},
		Presence:    careExitPresence{presence: presence},
		Calendar:    schoolCalendarCompose.NewCalendarPeriodIDs(calendar),
		Tags:        peopleCompose.NewStudentTagReleaser(people),
		ChangeTrail: changeTrail,
	}
}

// careEndRecorder serves the lifecycle's change-trail port: a moved last
// care day is one change of the child's tracked enrolment end.
type careEndRecorder struct{ audit StudentChangeAudit }

// NewCareEndRecorder binds the change trail to the student audit.
func NewCareEndRecorder(audit StudentChangeAudit) carePlanCompose.CareEndRecorder {
	return careEndRecorder{audit: audit}
}

func (r careEndRecorder) RecordCareEndChange(ctx context.Context, studentID int64, before, after *owner0.Date, actorAccountID int64) error {
	previous := &peopledirectory.StudentState{EnrolledUntil: before}
	previous.ID = studentID
	next := &peopledirectory.StudentState{EnrolledUntil: after}
	next.ID = studentID
	return r.audit.RecordChangesForActor(ctx, previous, next, actorAccountID)
}

// careExitBookings serves the lifecycle's booking port over the Timetable
// owner.
type careExitBookings struct{ capability timetable.Capability }

func (b careExitBookings) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil owner0.Date) error {
	return b.capability.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil.String())
}

func (b careExitBookings) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil owner0.Date) (carePlanCompose.CareExitBookingChanges, error) {
	changes, err := b.capability.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil.String())
	if err != nil {
		return carePlanCompose.CareExitBookingChanges{}, err
	}
	result := carePlanCompose.CareExitBookingChanges{
		Deleted: make([]carePlanCompose.CareExitBooking, 0, len(changes.Deleted)),
		Capped:  make([]carePlanCompose.CareExitBookingCap, 0, len(changes.Capped)),
	}
	for _, deleted := range changes.Deleted {
		result.Deleted = append(result.Deleted, careExitBooking(deleted))
	}
	for _, capped := range changes.Capped {
		result.Capped = append(result.Capped, carePlanCompose.CareExitBookingCap{
			StudentID: capped.StudentID, ID: capped.ID, PreviousValidUntil: careExitDay(capped.PreviousValidUntil),
		})
	}
	return result, nil
}

func (b careExitBookings) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, calendarPeriodIDs []int64, removals []carePlanCompose.CareExitBookingRestore) (int, error) {
	return b.capability.RestoreStudentEnrollmentsForCareExit(ctx, studentIDs, calendarPeriodIDs, careExitEnrollmentRemovals(removals))
}

func (b careExitBookings) CountRunningEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil owner0.Date, restorable []carePlanCompose.CareExitBookingRestore) (map[int64]int, error) {
	return b.capability.CountRunningEnrollmentsForCareExit(ctx, studentIDs, validUntil.String(), careExitEnrollmentRemovals(restorable))
}

func careExitBooking(value timetable.CareExitEnrollment) carePlanCompose.CareExitBooking {
	return carePlanCompose.CareExitBooking{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
		ValidFrom: owner0.Date(value.ValidFrom), ValidUntil: careExitDay(value.ValidUntil),
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
	}
}

func careExitEnrollmentRemovals(values []carePlanCompose.CareExitBookingRestore) []timetable.CareExitEnrollmentRemoval {
	result := make([]timetable.CareExitEnrollmentRemoval, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.CareExitEnrollmentRemoval{
			CareExitEnrollment: timetable.CareExitEnrollment{
				ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
				ValidFrom: value.ValidFrom.String(), ValidUntil: careExitDayString(value.ValidUntil),
				CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
				SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
			},
			WasDeleted: value.WasDeleted, PreviousValidUntil: careExitDayString(value.PreviousValidUntil),
		})
	}
	return result
}

// careExitPresence serves the lifecycle's presence port over Student
// Presence.
type careExitPresence struct {
	presence interface {
		ListOpenPresence(context.Context, []int64) ([]int64, error)
		LatestPresenceDate(context.Context, int64) (*string, error)
		LockOpenPresence(context.Context, []int64) error
		CloseOpenPresence(context.Context, []int64, time.Time) (int64, error)
	}
}

func (p careExitPresence) ListOpenPresence(ctx context.Context, studentIDs []int64) ([]int64, error) {
	return p.presence.ListOpenPresence(ctx, studentIDs)
}

func (p careExitPresence) LatestPresenceDate(ctx context.Context, studentID int64) (*owner0.Date, error) {
	value, err := p.presence.LatestPresenceDate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return parseCareExitDay(value)
}

func (p careExitPresence) LockOpenPresence(ctx context.Context, studentIDs []int64) error {
	return p.presence.LockOpenPresence(ctx, studentIDs)
}

func (p careExitPresence) CloseOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	return p.presence.CloseOpenPresence(ctx, studentIDs, at)
}

func parseCareExitDay(value *string) (*owner0.Date, error) {
	if value == nil {
		return nil, nil
	}
	day := peopleCompose.ParseRetainedCalendarDate(*value)
	if day == nil {
		return nil, fmt.Errorf("care exit: invalid calendar day %q", *value)
	}
	return day, nil
}

func careExitDay(value *string) *owner0.Date {
	if value == nil {
		return nil
	}
	day := owner0.Date(*value)
	return &day
}

func careExitDayString(value *owner0.Date) *string {
	if value == nil {
		return nil
	}
	day := value.String()
	return &day
}
