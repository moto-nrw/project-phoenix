package repositories

import (
	"context"
	"fmt"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

// CareLifecycleOwnerSources are the owners the Care Plan lifecycle (#3427)
// drives around a care exit: the child rows and memberships, the persons and
// bracelets, the timetable, the school calendar. Enrollment and Student
// Presence are composed from the database here.
type CareLifecycleOwnerSources struct {
	Students   users.StudentRepository
	Persons    users.PersonRepository
	Membership CareMembershipCommands
	People     peopledirectory.Capability
	Timetable  timetable.Capability
	Calendar   schoolcalendar.Query
}

// NewCareLifecycleOwners binds the lifecycle's ports to their owners. The
// change trail arrives from the caller, which holds the audit service.
func NewCareLifecycleOwners(db *bun.DB, sources CareLifecycleOwnerSources, changeTrail carePlanCompose.CareEndRecorder) carePlanCompose.CareLifecycleOwners {
	presence := newStudentPresence(db)
	return carePlanCompose.CareLifecycleOwners{
		Students: CareStudents{
			students: sources.Students, persons: sources.Persons,
			membership: sources.Membership, people: sources.People,
		},
		Roster:      newCareExitRoster(db, sources.Timetable, presence),
		Bookings:    careExitBookings{capability: sources.Timetable},
		Enrollment:  careExitEnrollment{projection: NewEnrollmentBookingProjection(enrollmentCompose.New())},
		Presence:    careExitPresence{presence: presence},
		Calendar:    careExitCalendarPeriods{calendar: sources.Calendar},
		Tags:        NewStudentTagReleaser(sources.People),
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

func (r careEndRecorder) RecordCareEndChange(ctx context.Context, studentID int64, before, after *users.CalendarDate, actorAccountID int64) error {
	previous := &users.Student{EnrolledUntil: before}
	previous.ID = studentID
	next := &users.Student{EnrolledUntil: after}
	next.ID = studentID
	return r.audit.RecordChangesForActor(ctx, previous, next, actorAccountID)
}

// careExitBookings serves the lifecycle's booking port over the Timetable
// owner.
type careExitBookings struct{ capability timetable.Capability }

func (b careExitBookings) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil users.CalendarDate) error {
	return b.capability.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil.String())
}

func (b careExitBookings) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil users.CalendarDate) (carePlanCompose.CareExitBookingChanges, error) {
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

func (b careExitBookings) CountRunningEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil users.CalendarDate, restorable []carePlanCompose.CareExitBookingRestore) (map[int64]int, error) {
	return b.capability.CountRunningEnrollmentsForCareExit(ctx, studentIDs, validUntil.String(), careExitEnrollmentRemovals(restorable))
}

func careExitBooking(value timetable.CareExitEnrollment) carePlanCompose.CareExitBooking {
	return carePlanCompose.CareExitBooking{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
		ValidFrom: users.CalendarDate(value.ValidFrom), ValidUntil: careExitDay(value.ValidUntil),
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

func (p careExitPresence) LatestPresenceDate(ctx context.Context, studentID int64) (*users.CalendarDate, error) {
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

func parseCareExitDay(value *string) (*users.CalendarDate, error) {
	if value == nil {
		return nil, nil
	}
	day := usersRepo.ParseCalendarDate(*value)
	if day == nil {
		return nil, fmt.Errorf("care exit: invalid calendar day %q", *value)
	}
	return day, nil
}

func careExitDay(value *string) *users.CalendarDate {
	if value == nil {
		return nil
	}
	day := users.CalendarDate(*value)
	return &day
}

func careExitDayString(value *users.CalendarDate) *string {
	if value == nil {
		return nil
	}
	day := value.String()
	return &day
}
