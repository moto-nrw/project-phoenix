package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The compositions the timetable route suites (modules/timetable/http) drive
// (#2732). They compose the Timetable owner over the retained repositories
// the composition root binds, so the suites import neither the repositories
// nor the retained models.

// TimetableHTTPTestRows are the retained repositories the timetable route
// suites arrange and assert through.
type TimetableHTTPTestRows = repositories.TimetableTestRepositories

// NewTimetableHTTPTestRows composes the retained repositories over the test
// database.
func NewTimetableHTTPTestRows(db *bun.DB, clocks ...func() time.Time) (TimetableHTTPTestRows, error) {
	return repositories.NewTimetableTestRepositories(db, clocks...)
}

// TimetableHTTPTestCareOfferingInvalid is what Care Plan's care-offering
// checks return for an offering a template change would invalidate; the
// composition classifies it as the conflict.
var TimetableHTTPTestCareOfferingInvalid = careplan.ErrCareOfferingConfigInvalid

// TimetableHTTPTestOptions are the collaborators a suite replaces: the
// Enrollment care-offering checks and roster resync (nil accepts), the
// materialization (nil = the owner's real engine over the test database), the
// instance lifecycle whose deviation machinery a split preserves (nil = no
// deviations) and the clock.
type TimetableHTTPTestOptions struct {
	ValidateCareOfferingSeries func(context.Context, int64) error
	ValidateOfferingSource     func(context.Context, []int64, []int64, *int64) error
	ResyncOfferingRoster       func(context.Context, timetable.OfferingRosterResyncInput) error
	Materialization            timetable.MaterializationCapability
	Instances                  timetable.InstanceLifecycleCapability
	Clock                      func() time.Time
}

// TimetableHTTPTestModule is what the composition root hands the timetable
// routes from the Timetable owner: the template writes, the planner reads,
// the conflict detection, the attendance correction with its audit trail and
// the recurrence gate.
type TimetableHTTPTestModule struct {
	Templates             timetable.TemplateAdministration
	Data                  timetable.TimetableDataCapability
	ConflictDetection     timetable.ConflictDetectionCapability
	AttendanceCorrections timetable.AttendanceCorrections
	RecurrenceLock        timetable.RecurrenceWriteLock
}

// NewTimetableHTTPTestModule composes the Timetable owner for the timetable
// route suites.
func NewTimetableHTTPTestModule(db *bun.DB, options TimetableHTTPTestOptions) (TimetableHTTPTestModule, error) {
	var clocks []func() time.Time
	if options.Clock != nil {
		clocks = append(clocks, options.Clock)
	}
	r, err := repositories.NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	var today func() calendar.Date
	if options.Clock != nil {
		clock := options.Clock
		today = func() calendar.Date { return calendar.DateFromTime(clock()) }
	}
	lock, err := repositories.NewTimetableRecurrenceLock(db)
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	recovery := repositories.NewActivityRecoveryRepository(db, r.InstanceStudent)
	rows := r.OwnerRows()
	rows.Locks = recovery
	conflicts, data, err := newTimetableHTTPTestReads(db, r, rows)
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	materialization := options.Materialization
	if materialization == nil {
		materialization, err = timetableCompose.NewMaterialization(timetableCompose.MaterializationDependencies{
			GroupRepo: r.ActivityGroup, ScheduleRepo: r.ActivitySchedule,
			EnrollmentRepo: r.StudentEnrollment, SupervisorRepo: r.ActivitySupervisor,
			PeriodRepo: r.CalendarPeriod, InstanceRepo: r.ActivityInstance, StaffRepo: r.InstanceStaff,
			StudentRepo: r.InstanceStudent, ExceptionRepo: r.ActivityException,
			TimeframeRepo: r.Timeframe, CareBounds: r.Student, RecurrenceLock: lock, DB: db,
		})
		if err != nil {
			return TimetableHTTPTestModule{}, err
		}
	}
	templates, err := timetableCompose.NewTemplateAdministration(timetableCompose.TemplateAdministrationDependencies{
		Groups:               r.ActivityGroup,
		Categories:           r.ActivityCategory,
		Schedules:            r.ActivitySchedule,
		Enrollments:          r.StudentEnrollment,
		Supervisors:          r.ActivitySupervisor,
		Instances:            r.ActivityInstance,
		InstanceStaff:        r.InstanceStaff,
		Participants:         r.InstanceStudent,
		Timeframes:           r.Timeframe,
		EducationGroups:      r.Group,
		Materialization:      materialization,
		Deviations:           timetableHTTPTestSeriesDeviations(options.Instances),
		CareOfferings:        timetableHTTPTestCareOfferingChecks(options),
		ResyncOfferingRoster: options.ResyncOfferingRoster,
		RecurrenceLock:       lock,
		SchoolClasses: timetableCompose.SchoolClassRules{
			MinGradeLevel: timetable.MinSchoolGradeLevel, MaxGradeLevel: timetable.MaxSchoolGradeLevel,
			Normalize: func(class string) string { return strings.ToLower(strings.TrimSpace(class)) },
		},
		DB:    db,
		Today: today,
	})
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	trail := func(ctx context.Context) (bun.IDB, int64) { return db, auditModels.TenantIDFromContext(ctx) }
	corrections, err := timetableCompose.NewAttendanceCorrections(timetableCompose.AttendanceCorrectionDependencies{
		Participants: r.InstanceStudent,
		Instances:    r.ActivityInstance,
		Trail:        repositories.TimetableAttendanceCorrectionTrail(repositories.NewAttendanceCorrectionRepository(trail)),
		People:       r.Person,
		Locks:        recovery,
	})
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	return TimetableHTTPTestModule{
		Templates:             templates,
		Data:                  data,
		ConflictDetection:     conflicts,
		AttendanceCorrections: corrections,
		RecurrenceLock:        lock,
	}, nil
}

// newTimetableHTTPTestReads composes the conflict detection and the planner
// reads over Care Plan's baselines in the legacy booking mode.
func newTimetableHTTPTestReads(db *bun.DB, r TimetableHTTPTestRows, rows repositories.TimetableOwnerRows) (timetable.ConflictDetectionCapability, timetable.TimetableDataCapability, error) {
	people, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return nil, nil, err
	}
	carePlan, err := repositories.NewCarePlan(db, people, r.InstanceStudent)
	if err != nil {
		return nil, nil, err
	}
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, r.Enrollment())
	if err != nil {
		return nil, nil, err
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return nil, nil, err
	}
	classArrivalQueries, err := timetableCompose.NewClassArrivalQueries(db, func(timetableCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	legacyBookings := func(context.Context) (bool, error) { return false, nil }
	arrivals, err := NewArrivalBaselines(carePlan, people, classArrivalQueries, approvedOfferings, legacyBookings)
	if err != nil {
		return nil, nil, err
	}
	pickups, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, legacyBookings)
	if err != nil {
		return nil, nil, err
	}
	conflicts, err := timetableCompose.NewConflictDetection(timetableCompose.ConflictDetectionDependencies{
		Instances:         r.ActivityInstance,
		InstanceStaff:     r.InstanceStaff,
		InstanceStudents:  r.InstanceStudent,
		Exceptions:        r.ActivityException,
		Schedules:         r.ActivitySchedule,
		Shifts:            r.StaffShift,
		Staff:             r.Staff,
		CalendarPeriods:   r.CalendarPeriod,
		ArrivalExceptions: r.StudentArrivalException,
		ArrivalBaselines:  arrivalBaselineProjector{reader: arrivals},
		Presence:          presence,
		Sessions:          r.ActiveGroup,
		ContentHash:       securityruntime.Fingerprint,
	})
	if err != nil {
		return nil, nil, err
	}
	templates, ok := r.ActivityGroup.(timetableCompose.DataTemplates)
	if !ok {
		return nil, nil, fmt.Errorf("timetable data: %T cannot read the template list", r.ActivityGroup)
	}
	data, err := timetableCompose.NewTimetableData(timetableCompose.TimetableDataDependencies{
		Instances:         r.ActivityInstance,
		InstanceStaff:     r.InstanceStaff,
		Participants:      r.InstanceStudent,
		PickupExceptions:  r.StudentPickupException,
		ArrivalExceptions: r.StudentArrivalException,
		ArrivalBaselines:  arrivalBaselineProjector{reader: arrivals},
		PickupBaselines:   pickupBaselineProjector{reader: pickups},
		Visits:            presence,
		Templates:         templates,
		Groups:            r.Timetable,
		Categories:        r.ActivityCategory,
		Rooms:             rows.RoomNames(),
		RoomOccupancy:     timetableRoomOccupancy{sessions: r.ActiveGroup},
		DeviationEvents:   rows.DeviationEventReader(),
		ConflictAcks:      r.Timetable,
		Locks:             rows.AttendanceLocks(),
		Advisory:          newTenantAdvisoryLocks(true),
	})
	if err != nil {
		return nil, nil, err
	}
	return conflicts, data, nil
}

// timetableHTTPTestCareOfferingChecks binds the suite's care-offering
// callbacks; an omitted callback accepts, and an invalid linked offering is
// the conflict, as the composition root binds the Care Plan catalog.
func timetableHTTPTestCareOfferingChecks(options TimetableHTTPTestOptions) timetableCompose.CareOfferingChecks {
	checks := timetableCompose.CareOfferingChecks{
		ValidateSeries:         options.ValidateCareOfferingSeries,
		ValidateOfferingSource: options.ValidateOfferingSource,
		IsConflict: func(err error) bool {
			return isCareOfferingConfigInvalid(err)
		},
	}
	if checks.ValidateSeries == nil {
		checks.ValidateSeries = func(context.Context, int64) error { return nil }
	}
	if checks.ValidateOfferingSource == nil {
		checks.ValidateOfferingSource = func(context.Context, []int64, []int64, *int64) error { return nil }
	}
	return checks
}

// timetableHTTPTestSeriesDeviations binds the split's deviation port to the
// suite's instance lifecycle, or to one that preserves nothing.
func timetableHTTPTestSeriesDeviations(instances timetable.InstanceLifecycleCapability) timetableCompose.SeriesDeviations {
	if instances == nil {
		return timetableHTTPTestNoDeviations{}
	}
	lifecycle, ok := instances.(interface {
		SeriesDeviations() timetableCompose.SeriesDeviations
	})
	if !ok {
		panic(fmt.Sprintf("timetable data: %T cannot preserve series deviations", instances))
	}
	return lifecycle.SeriesDeviations()
}

type timetableHTTPTestNoDeviations struct{}

func (timetableHTTPTestNoDeviations) LockDeviationDays(context.Context, int64, calendar.Date, calendar.Date) error {
	return nil
}

func (timetableHTTPTestNoDeviations) SnapshotDeviations(context.Context, calendar.Date, calendar.Date, int64) (timetableCompose.PreservedDeviations, error) {
	return timetableHTTPTestNoPreservedDeviations{}, nil
}

type timetableHTTPTestNoPreservedDeviations struct{}

func (timetableHTTPTestNoPreservedDeviations) Count() int { return 0 }

func (timetableHTTPTestNoPreservedDeviations) Reapply(context.Context, int64, *int64) (int, error) {
	return 0, nil
}

// NewTimetableHTTPTestSessionRecords are Student Presence's live sessions and
// supervisions over the test database, for suites that arrange a running
// block.
func NewTimetableHTTPTestSessionRecords(db *bun.DB) *presenceCompose.SessionRecords {
	return repositories.NewPresenceSessionRecords(db)
}

// NewTimetableHTTPTestRecurrenceLock is the Timetable owner's tenant
// recurrence gate over the test database.
func NewTimetableHTTPTestRecurrenceLock(db *bun.DB) (timetable.RecurrenceWriteLock, error) {
	return repositories.NewTimetableRecurrenceLock(db)
}

// NewTimetableHTTPTestPeople serves the People port of the timetable routes
// over the person, staff and student rows of the test database.
func NewTimetableHTTPTestPeople(db *bun.DB) (TimetablePeople, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return TimetablePeople{}, err
	}
	return NewTimetablePeople(users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo: r.Person, StaffRepo: r.Staff, StudentRepo: r.Student,
	})), nil
}

// NewTimetableHTTPTestOfferingSources serves the offering-source support of
// the timetable routes from the enrollment decision service over the test
// database.
func NewTimetableHTTPTestOfferingSources(db *bun.DB, unit tenant.UnitOfWork) (timetable.OfferingSourceSupport, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return nil, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, err
	}
	support := NewTimetableOfferingSources(enrollment.NewDecisionService(enrollment.DecisionServiceConfig{
		CareOfferingRepo:  enrollment.NewCareOfferingRepository(r.CarePlan),
		ActivityGroupRepo: r.ActivityGroup,
		Settings:          settings.Settings,
	}))
	if support == nil {
		return nil, errors.New("enrollment decision service serves no offering sources")
	}
	return support, nil
}
