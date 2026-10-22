//go:build testhooks

package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	blockDisplayCompose "github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The compositions the timetable route suites (modules/timetable/http) drive
// (#2732). They compose the Timetable owner over the retained repositories
// the composition root binds, so the suites import neither the repositories
// nor the retained models.

// TimetableHTTPTestRows are the retained repositories the timetable route
// suites arrange and assert through.
type TimetableHTTPTestRows = owners.TimetableTestRepositories

// NewTimetableHTTPTestRows composes the retained repositories over the test
// database.
func NewTimetableHTTPTestRows(db transactionruntime.Database, clocks ...func() time.Time) (TimetableHTTPTestRows, error) {
	return NewTimetableTestRepositories(db, clocks...)
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
func NewTimetableHTTPTestModule(db transactionruntime.Database, options TimetableHTTPTestOptions) (TimetableHTTPTestModule, error) {
	var clocks []func() time.Time
	if options.Clock != nil {
		clocks = append(clocks, options.Clock)
	}
	r, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	members := owners.MustNewMembershipTestRepositories(db)
	var today func() calendar.Date
	if options.Clock != nil {
		clock := options.Clock
		today = func() calendar.Date { return calendar.DateFromTime(clock()) }
	}
	lock, err := owners.NewTimetableTestRecurrenceLock(db)
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	recovery := timetableCompose.NewRetainedActivityRecoveryRepository(db.Pool(), r.InstanceStudent)
	rows := r.OwnerRows()
	rows.Locks = recovery
	conflicts, data, err := newTimetableHTTPTestReads(db, r, members, rows)
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	materialization := options.Materialization
	if materialization == nil {
		materialization, err = timetableCompose.NewMaterialization(timetablePorts.MaterializationDependencies{
			GroupRepo: r.Timetable, ScheduleRepo: r.ActivitySchedule,
			EnrollmentRepo: r.StudentEnrollment, SupervisorRepo: r.ActivitySupervisor, Directories: r.Directories,
			PeriodRepo: r.CalendarPeriod, InstanceRepo: r.ActivityInstance, StaffRepo: r.InstanceStaff,
			StudentRepo: r.InstanceStudent, ExceptionRepo: r.ActivityException,
			TimeframeRepo: r.Timeframe, CareBounds: members.Student, RecurrenceLock: lock, Transactions: timetableCompose.NewTenantTransactions(db.Pool()),
		})
		if err != nil {
			return TimetableHTTPTestModule{}, err
		}
	}
	templates, err := timetableCompose.NewTemplateAdministration(timetablePorts.TemplateAdministrationDependencies{
		Groups:               r.Timetable,
		Categories:           r.ActivityCategory,
		Schedules:            r.ActivitySchedule,
		Enrollments:          r.StudentEnrollment,
		Supervisors:          r.ActivitySupervisor,
		Directories:          r.Directories,
		Instances:            r.ActivityInstance,
		InstanceStaff:        r.InstanceStaff,
		Participants:         r.InstanceStudent,
		Timeframes:           r.Timeframe,
		EducationGroups:      owners.NewSchoolStructureSuiteRepositories(db).Group,
		Materialization:      materialization,
		Deviations:           timetableHTTPTestSeriesDeviations(options.Instances),
		CareOfferings:        timetableHTTPTestCareOfferingChecks(options),
		ResyncOfferingRoster: options.ResyncOfferingRoster,
		RecurrenceLock:       lock,
		SchoolClasses: timetablePorts.SchoolClassRules{
			MinGradeLevel: timetable.MinSchoolGradeLevel, MaxGradeLevel: timetable.MaxSchoolGradeLevel,
			Normalize: func(class string) string { return strings.ToLower(strings.TrimSpace(class)) },
		},
		Transactions: timetableCompose.NewTenantTransactions(db.Pool()),
		Today:        today,
	})
	if err != nil {
		return TimetableHTTPTestModule{}, err
	}
	trail := auditCompose.NewFixedRuntime(db.Pool(), auditModels.TenantIDFromContext)
	corrections, err := timetableCompose.NewAttendanceCorrections(timetablePorts.AttendanceCorrectionDependencies{
		Participants: r.InstanceStudent,
		Instances:    r.ActivityInstance,
		Trail:        TimetableAttendanceCorrectionTrail(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{Reads: trail}).AttendanceCorrection),
		People:       timetableCorrectionPeople{rows: members.Person},
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
func newTimetableHTTPTestReads(db transactionruntime.Database, r TimetableHTTPTestRows, members owners.MembershipTestRepositories, rows timetableCompose.OwnerRows) (timetable.ConflictDetectionCapability, timetable.TimetableDataCapability, error) {
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return nil, nil, err
	}
	sessions, err := NewTimetablePresenceRows(db, r, members)
	if err != nil {
		return nil, nil, err
	}
	carePlan, err := owners.NewCarePlan(db, people, r.InstanceStudent)
	if err != nil {
		return nil, nil, err
	}
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, r.Enrollment())
	if err != nil {
		return nil, nil, err
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return nil, nil, err
	}
	classArrivalQueries, err := timetableCompose.NewClassArrivalQueries(db.Pool(), func(timetableCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	legacyBookings := func(context.Context) (bool, error) { return false, nil }
	arrivals, err := owners.NewArrivalBaselines(carePlan, people, classArrivalQueries, approvedOfferings, legacyBookings)
	if err != nil {
		return nil, nil, err
	}
	pickups, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, legacyBookings)
	if err != nil {
		return nil, nil, err
	}
	conflicts, err := timetableCompose.NewConflictDetection(timetablePorts.ConflictDetectionDependencies{
		Instances:         r.ActivityInstance.Reads,
		InstanceStaff:     r.InstanceStaff,
		InstanceStudents:  r.InstanceStudent,
		Exceptions:        r.ActivityException,
		Schedules:         r.ActivitySchedule,
		Shifts:            r.StaffShift,
		Staff:             owners.NewTimetableStaff(members.Staff),
		CalendarPeriods:   r.CalendarPeriod,
		ArrivalExceptions: r.StudentArrivalException,
		ArrivalBaselines:  owners.ArrivalBaselineProjector{Reader: arrivals},
		Presence:          presence,
		Sessions:          sessions.ActiveGroup,
		ContentHash:       securityruntime.Fingerprint,
	})
	if err != nil {
		return nil, nil, err
	}
	data, err := timetableCompose.NewTimetableData(timetablePorts.TimetableDataDependencies{
		Instances:         r.ActivityInstance.Reads,
		InstanceStaff:     r.InstanceStaff,
		Participants:      r.InstanceStudent,
		PickupExceptions:  r.StudentPickupException,
		ArrivalExceptions: r.StudentArrivalException,
		ArrivalBaselines:  owners.ArrivalBaselineProjector{Reader: arrivals},
		PickupBaselines:   PickupBaselineProjector{Reader: pickups},
		Visits:            presence,
		Templates:         r.Timetable,
		Directories:       r.Directories,
		Groups:            r.Timetable,
		BlockMetadata:     timetableCompose.NewBlockDisplayMetadata(db.Pool(), blockDisplayCompose.List),
		Categories:        r.ActivityCategory,
		Rooms:             rows.Rooms,
		RoomOccupancy:     TimetableRoomOccupancy{Sessions: sessions.ActiveGroup},
		DeviationEvents:   rows.DeviationEvents,
		ConflictAcks:      r.Timetable,
		Locks:             rows.Locks,
		Advisory:          timetableAdvisoryLocks{},
	})
	if err != nil {
		return nil, nil, err
	}
	return conflicts, data, nil
}

// timetableHTTPTestCareOfferingChecks binds the suite's care-offering
// callbacks; an omitted callback accepts, and an invalid linked offering is
// the conflict, as the composition root binds the Care Plan catalog.
func timetableHTTPTestCareOfferingChecks(options TimetableHTTPTestOptions) timetablePorts.CareOfferingChecks {
	checks := timetablePorts.CareOfferingChecks{
		ValidateSeries:         options.ValidateCareOfferingSeries,
		ValidateOfferingSource: options.ValidateOfferingSource,
		IsConflict: func(err error) bool {
			return IsCareOfferingConfigInvalid(err)
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
func timetableHTTPTestSeriesDeviations(instances timetable.InstanceLifecycleCapability) timetablePorts.SeriesDeviations {
	if instances == nil {
		return timetableHTTPTestNoDeviations{}
	}
	lifecycle, ok := instances.(interface {
		SeriesDeviations() timetablePorts.SeriesDeviations
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

func (timetableHTTPTestNoDeviations) SnapshotDeviations(context.Context, calendar.Date, calendar.Date, int64) (timetablePorts.PreservedDeviations, error) {
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
func NewTimetableHTTPTestSessionRecords(db transactionruntime.Database) presenceCompose.SessionRecords {
	return NewPresenceSessionRecords(db)
}

// NewTimetableHTTPTestRecurrenceLock is the Timetable owner's tenant
// recurrence gate over the test database.
func NewTimetableHTTPTestRecurrenceLock(db transactionruntime.Database) (timetable.RecurrenceWriteLock, error) {
	return owners.NewTimetableTestRecurrenceLock(db)
}

// NewTimetableHTTPTestPeople serves the People port of the timetable routes
// over the person, staff and student rows of the test database.
func NewTimetableHTTPTestPeople(db transactionruntime.Database) (TimetablePeople, error) {
	r, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		return TimetablePeople{}, err
	}
	return NewTimetablePeople(peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonRepo: r.Person, StudentRepo: r.Student,
		StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: r.Person, Staff: r.Staff, MayManageStaff: StaffAdministrationPermitted}),
	})), nil
}

// NewTimetableHTTPTestOfferingSources serves the offering-source support of
// the timetable routes from Care Plan's booking materialization over the test
// database.
func NewTimetableHTTPTestOfferingSources(db transactionruntime.Database, unit tenant.UnitOfWork) (timetable.OfferingSourceSupport, error) {
	module, err := NewBookingMaterializationTestModule(db, unit, BookingMaterializationTestOptions{})
	if err != nil {
		return nil, err
	}
	support := NewTimetableOfferingSources(module.Bookings)
	if support == nil {
		return nil, errors.New("care plan serves no offering sources")
	}
	return support, nil
}

// TimetableHTTPOption replaces one collaborator of the Timetable owner the
// timetable route suites drive.
type TimetableHTTPOption func(*TimetableHTTPTestOptions)

// WithCareOfferingSeriesValidator replaces Care Plan's check of a template
// linked to a care offering.
func WithCareOfferingSeriesValidator(validate func(context.Context, int64) error) TimetableHTTPOption {
	return func(options *TimetableHTTPTestOptions) { options.ValidateCareOfferingSeries = validate }
}

// WithOfferingSourceValidator replaces Care Plan's check of a template's
// offering sources.
func WithOfferingSourceValidator(validate func(context.Context, []int64, []int64, *int64) error) TimetableHTTPOption {
	return func(options *TimetableHTTPTestOptions) { options.ValidateOfferingSource = validate }
}

// WithOfferingRosterResync replaces Enrollment's roster resync of an
// offering-sourced template.
func WithOfferingRosterResync(resync func(context.Context, timetable.OfferingRosterResyncInput) error) TimetableHTTPOption {
	return func(options *TimetableHTTPTestOptions) { options.ResyncOfferingRoster = resync }
}

// WithTimetableMaterialization replaces the owner's materialization.
func WithTimetableMaterialization(materialization timetable.MaterializationCapability) TimetableHTTPOption {
	return func(options *TimetableHTTPTestOptions) { options.Materialization = materialization }
}

// WithTimetableInstances hands the template split the instance lifecycle
// whose deviations it preserves.
func WithTimetableInstances(instances timetable.InstanceLifecycleCapability) TimetableHTTPOption {
	return func(options *TimetableHTTPTestOptions) { options.Instances = instances }
}

// WithTimetableClock replaces the owner's clock.
func WithTimetableClock(clock func() time.Time) TimetableHTTPOption {
	return func(options *TimetableHTTPTestOptions) { options.Clock = clock }
}

// NewTimetableHTTP composes the Timetable owner the timetable routes drive:
// template writes, planner reads, conflict detection, attendance correction
// and the recurrence gate.
func NewTimetableHTTP(db transactionruntime.Database, options ...TimetableHTTPOption) (TimetableHTTPTestModule, error) {
	var composed TimetableHTTPTestOptions
	for _, option := range options {
		option(&composed)
	}
	return NewTimetableHTTPTestModule(db, composed)
}
