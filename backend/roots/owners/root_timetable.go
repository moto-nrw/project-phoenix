package owners

import (
	"fmt"
	"log/slog"
	"time"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	blockDisplayCompose "github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimetableOwner is Timetable & Activities as the roots bind it (#4009): the
// retained activities and schedule repositories (#2743), School Structure's
// group names for the owner rows, the planning-track administration, the
// tenant recurrence gate every recurrence writer serializes on (#3424 slice
// S2), the template rows of the materialization and the roster reconciler
// of the grade transitions (#405). BlockMetadata is the block display
// projection naming each planned block's room and planning track (#4413).
type TimetableOwner struct {
	Repositories        TimetableRepositories
	EducationGroupNames timetablePorts.EducationGroupNames
	PlanningTracks      timetable.PlanningTrackAdministration
	RecurrenceLock      timetable.RecurrenceWriteLock
	TemplateRows        arrivalTimetable.TemplateRows
	RosterReconciler    timetablePorts.RosterReconciler
	BlockMetadata       timetablePorts.BlockDisplayMetadata
	// StudentTemplateDays names and removes the hand planning a booking
	// change leaves behind (#5102).
	StudentTemplateDays timetable.StudentTemplateDays
}

// BuildTimetableOwner binds the Timetable owner's retained rows. The
// calendar periods of the template rows come from the School Calendar with
// Enrollment's period usage.
func BuildTimetableOwner(
	db transactionruntime.Database, logger *slog.Logger, now func() time.Time,
	owner timetable.Capability, repos TimetableRepositories, calendar schoolcalendar.Days,
	enrollment *enrollmentOwner.BookingProjection, groups EducationGroupRows,
) (TimetableOwner, error) {
	pool := db.Pool()
	recurrenceLock, err := arrivalTimetable.NewRecurrenceWriteLock(arrivalTimetable.RecurrenceLockDependencies{
		DB: pool, GradeTransitionLockKey: schoolstructure.TransitionsLockKey,
	})
	if err != nil {
		return TimetableOwner{}, fmt.Errorf("compose timetable recurrence lock: %w", err)
	}
	calendarPeriods := arrivalTimetable.NewCalendarPeriodRepository(calendar, arrivalTimetable.NewPlanningCalendarPeriodUsage(enrollment.Owner, owner))
	today := timezone.TodayDate
	if now != nil {
		today = func() timezone.Date { return timezone.DateFromTime(now()) }
	}
	studentDays, err := NewTimetableStudentTemplateDays(repos, recurrenceLock, today, logger)
	if err != nil {
		return TimetableOwner{}, fmt.Errorf("compose timetable student template days: %w", err)
	}
	return TimetableOwner{
		Repositories:        repos,
		EducationGroupNames: TimetableEducationGroupNames(groups),
		PlanningTracks:      arrivalTimetable.NewPlanningTrackAdministration(owner, pool),
		RecurrenceLock:      recurrenceLock,
		TemplateRows: arrivalTimetable.TemplateRows{
			Groups: repos.Timetable, Categories: repos.ActivityCategory, Schedules: repos.ActivitySchedule,
			Enrollments: repos.StudentEnrollment, Supervisors: repos.ActivitySupervisor, Directories: repos.Directories, Instances: repos.ActivityInstance,
			InstanceStaff: repos.InstanceStaff, Participants: repos.InstanceStudent, Timeframes: repos.Timeframe,
			CalendarPeriods: calendarPeriods, Exceptions: repos.ActivityException, EducationGroups: groups,
		},
		// Reconciles already-materialized future timetable rosters when a
		// grade transition graduates or restores students (#405).
		RosterReconciler:    arrivalTimetable.NewRosterReconciler(repos.ActivityInstance, repos.InstanceStudent, repos.StudentEnrollment, repos.Directories, logger, now),
		BlockMetadata:       arrivalTimetable.NewBlockDisplayMetadata(pool, blockDisplayCompose.List),
		StudentTemplateDays: studentDays,
	}, nil
}

// NewTimetableStudentTemplateDays binds the student template days over the
// retained rows and the recurrence gate; today bounds the removal.
func NewTimetableStudentTemplateDays(
	repos TimetableRepositories, lock timetable.RecurrenceWriteLock, today func() timezone.Date, logger *slog.Logger,
) (timetable.StudentTemplateDays, error) {
	return arrivalTimetable.NewStudentTemplateDays(timetablePorts.StudentTemplateDaysDependencies{
		Groups: repos.Timetable, Schedules: repos.ActivitySchedule, Timeframes: repos.Timeframe, Enrollments: repos.StudentEnrollment,
		Instances: repos.ActivityInstance.Reads, Participants: repos.InstanceStudent, RecurrenceLock: lock,
		Today: today, Logger: logger,
	})
}
