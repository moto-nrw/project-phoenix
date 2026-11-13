package owners

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-ogs/project-phoenix/realtime"

	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	"github.com/moto-ogs/project-phoenix/modules/schoolstructure"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-ogs/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	blockDisplayCompose "github.com/moto-ogs/project-phoenix/modules/timetableblockdisplay/compose"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	// CohortRosters moves children who change Klasse or Gruppe between the
	// materialized occurrences of dynamically targeted Regeltermine (#6841).
	CohortRosters timetable.CohortRosters
}

// BuildTimetableOwner binds the Timetable owner's retained rows. The
// calendar periods of the template rows come from the School Calendar with
// Enrollment's period usage.
func BuildTimetableOwner(
	db transactionruntime.Database, logger *slog.Logger, now func() time.Time,
	owner timetable.Capability, repos TimetableRepositories, calendar schoolcalendar.Days,
	enrollment *enrollmentOwner.BookingProjection, groups EducationGroupRows, broadcaster realtime.Broadcaster,
	students peopledirectory.StudentQuery, careBounds timetablePorts.CareBoundReader,
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
	studentDays, err := NewTimetableStudentTemplateDays(repos, calendarPeriods, recurrenceLock, today, logger, NewStaffingAnnouncer(broadcaster))
	if err != nil {
		return TimetableOwner{}, fmt.Errorf("compose timetable student template days: %w", err)
	}
	cohorts, err := NewTimetableCohortRosters(repos, students, careBounds, logger)
	if err != nil {
		return TimetableOwner{}, fmt.Errorf("compose timetable cohort rosters: %w", err)
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
		CohortRosters:       cohorts,
	}, nil
}

// NewTimetableCohortRosters binds the cohort roster follow-up (#6841) over
// the retained rows and the People Directory's enrolled children; careBounds
// may be nil.
func NewTimetableCohortRosters(
	repos TimetableRepositories, students peopledirectory.StudentQuery, careBounds timetablePorts.CareBoundReader, logger *slog.Logger,
) (timetable.CohortRosters, error) {
	return arrivalTimetable.NewCohortRosters(timetablePorts.CohortRosterDependencies{
		Groups: repos.Timetable, Students: timetableStudents(students), Enrollments: repos.StudentEnrollment, Directories: repos.Directories,
		CareBounds: careBounds, Instances: repos.ActivityInstance, Participants: repos.InstanceStudent, Logger: logger,
	})
}

// FollowClassChanges binds a class-only follow-up of the cohort rosters: the
// writer names each child's class before the write; the group is unchanged.
func FollowClassChanges(rosters timetable.CohortRosters) func(context.Context, map[int64]string, timezone.Date) error {
	return func(ctx context.Context, previousClasses map[int64]string, from timezone.Date) error {
		changes := make([]timetable.CohortChange, 0, len(previousClasses))
		for studentID, previous := range previousClasses {
			changes = append(changes, timetable.CohortChange{StudentID: studentID, SchoolClassChanged: true, PreviousSchoolClass: previous})
		}
		return rosters.FollowCohortChanges(ctx, changes, from)
	}
}

// NewTimetableStudentTemplateDays binds the student template days over the
// retained rows and the recurrence gate; today bounds the removal, staffing
// wakes the planning caches (nil wakes none).
func NewTimetableStudentTemplateDays(
	repos TimetableRepositories, periods timetablePorts.ActiveCalendarPeriods, lock timetable.RecurrenceWriteLock, today func() timezone.Date, logger *slog.Logger,
	staffing timetablePorts.StaffingAnnouncer,
) (timetable.StudentTemplateDays, error) {
	return arrivalTimetable.NewStudentTemplateDays(timetablePorts.StudentTemplateDaysDependencies{
		Groups: repos.Timetable, Schedules: repos.ActivitySchedule, Timeframes: repos.Timeframe, Periods: periods, Enrollments: repos.StudentEnrollment,
		Instances: repos.ActivityInstance.Reads, Participants: repos.InstanceStudent, RecurrenceLock: lock,
		Staffing: staffing, Today: today, Logger: logger,
	})
}
