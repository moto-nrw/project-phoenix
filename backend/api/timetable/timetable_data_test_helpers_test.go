package timetable

import (
	"context"
	"errors"
	"fmt"
	"time"

	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetablesqltest"
)

// testTimetable bundles what the composition root hands api/timetable from
// the Timetable owner: the template writes (embedded, so the value serves
// Dependencies.Templates directly), the planner reads, the conflict
// detection and the retained attendance correction.
type testTimetable struct {
	timetableModule.TemplateAdministration
	data        timetableModule.TimetableDataCapability
	conflicts   timetableModule.ConflictDetectionCapability
	corrections timetableModule.AttendanceCorrections
	lock        timetableModule.RecurrenceWriteLock
}

func (t *testTimetable) TimetableData() timetableModule.TimetableDataCapability { return t.data }

func (t *testTimetable) ConflictDetection() timetableModule.ConflictDetectionCapability {
	return t.conflicts
}

func (t *testTimetable) AttendanceCorrections() timetableModule.AttendanceCorrections {
	return t.corrections
}

func (t *testTimetable) RecurrenceLock() timetableModule.RecurrenceWriteLock { return t.lock }

// testTimetableOptions are the collaborators a suite replaces: Enrollment's
// care-offering checks and roster resync, the materialization (nil = the
// owner's real engine over the test database) and the instance lifecycle
// whose deviation machinery a split preserves (nil = no deviations).
type testTimetableOptions struct {
	validateCareOfferingSeries func(context.Context, int64) error
	validateOfferingSource     func(context.Context, []int64, []int64, *int64) error
	resyncOfferingRoster       func(context.Context, timetableModule.OfferingRosterResyncInput) error
	materialization            timetableModule.MaterializationCapability
	instances                  timetableplanning.InstanceService
}

// testTimetableData builds the Timetable owner's template writes, planner
// reads (TimetableData()) and conflict detection (ConflictDetection())
// against the test database — the test-side equivalent of the factory
// wiring.
func testTimetableData(db *bun.DB, clocks ...func() time.Time) *testTimetable {
	return testTimetableDataWithCareValidator(db, nil, clocks...)
}

func testTimetableDataWithCareValidator(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	clocks ...func() time.Time,
) *testTimetable {
	return testTimetableDataWithOfferingCallbacks(db, validateCareOfferingSeries, nil, nil, clocks...)
}

func testTimetableDataWithOfferingCallbacks(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	validateOfferingSource func(context.Context, []int64, []int64, *int64) error,
	resyncOfferingRoster func(context.Context, timetableModule.OfferingRosterResyncInput) error,
	clocks ...func() time.Time,
) *testTimetable {
	return testTimetableWith(db, testTimetableOptions{
		validateCareOfferingSeries: validateCareOfferingSeries,
		validateOfferingSource:     validateOfferingSource,
		resyncOfferingRoster:       resyncOfferingRoster,
	}, clocks...)
}

func testTimetableWith(db *bun.DB, options testTimetableOptions, clocks ...func() time.Time) *testTimetable {
	boundRepos := mustTimetableTestRepositories(db, clocks...)
	people, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		panic(err)
	}
	carePlan, err := repositories.NewCarePlan(db, people, boundRepos.InstanceStudent)
	if err != nil {
		panic(err)
	}
	approvedOfferings, err := testutil.NewApprovedOfferingProjection(db, boundRepos.Enrollment())
	if err != nil {
		panic(err)
	}
	activityInstanceRepo := timetablesqltest.NewActivityInstanceRepository(db)
	var today func() timezone.Date
	if len(clocks) > 0 && clocks[0] != nil {
		clock := clocks[0]
		today = func() timezone.Date { return timezone.DateFromTime(clock()) }
		activityInstanceRepo = timetablesqltest.NewActivityInstanceRepository(db, clock)
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		panic(err)
	}
	classArrivalQueries, err := arrivalTimetable.NewClassArrivalQueries(db, func(arrivalTimetable.Observation) {})
	if err != nil {
		panic(err)
	}
	arrivalBaselines, err := newArrivalBaselinesFixture(carePlan, people, classArrivalQueries, approvedOfferings, func(context.Context) (bool, error) { return false, nil })
	if err != nil {
		panic(err)
	}
	conflicts, err := arrivalTimetable.NewConflictDetection(arrivalTimetable.ConflictDetectionDependencies{
		Instances:         activityInstanceRepo,
		InstanceStaff:     timetablesqltest.NewInstanceStaffRepository(db),
		InstanceStudents:  boundRepos.InstanceStudent,
		Exceptions:        timetablesqltest.NewActivityExceptionRepository(db),
		Schedules:         boundRepos.ActivitySchedule,
		Shifts:            boundRepos.StaffShift,
		Staff:             boundRepos.Staff,
		CalendarPeriods:   boundRepos.CalendarPeriod,
		ArrivalExceptions: boundRepos.StudentArrivalException,
		ArrivalBaselines:  testArrivalBaselines{reader: arrivalBaselines},
		Presence:          presence,
		Sessions:          boundRepos.ActiveGroup,
		ContentHash:       securityruntime.Fingerprint,
	})
	if err != nil {
		panic(err)
	}
	recovery := repositories.NewActivityRecoveryRepository(db, boundRepos.InstanceStudent)
	templates, ok := boundRepos.ActivityGroup.(arrivalTimetable.DataTemplates)
	if !ok {
		panic(fmt.Sprintf("timetable data: %T cannot read the template list", boundRepos.ActivityGroup))
	}
	timetableData, err := arrivalTimetable.NewTimetableData(arrivalTimetable.TimetableDataDependencies{
		Instances:         activityInstanceRepo,
		InstanceStaff:     timetablesqltest.NewInstanceStaffRepository(db),
		Participants:      boundRepos.InstanceStudent,
		PickupExceptions:  boundRepos.StudentPickupException,
		ArrivalExceptions: boundRepos.StudentArrivalException,
		ArrivalBaselines:  testArrivalBaselines{reader: arrivalBaselines},
		PickupBaselines:   testPickupBaselines{reader: newPickupBaselineService(carePlan, approvedOfferings)},
		Visits:            presence,
		Templates:         templates,
		Groups:            boundRepos.Timetable,
		Categories:        boundRepos.ActivityCategory,
		Rooms:             testRoomNames{rooms: boundRepos.Room},
		RoomOccupancy:     testRoomOccupancy{sessions: boundRepos.ActiveGroup},
		DeviationEvents: testDeviationEvents{
			events: auditRepo.NewDeviationEventRepository(auditRepo.NewRuntime(db, auditModels.TenantIDFromContext)),
		},
		ConflictAcks: boundRepos.Timetable,
		Locks:        recovery,
		Advisory:     testAdvisoryLocks{},
	})
	if err != nil {
		panic(err)
	}
	lock := repositories.MustNewTimetableRecurrenceLock(db)
	instanceStaff := timetablesqltest.NewInstanceStaffRepository(db)
	materialization := options.materialization
	if materialization == nil {
		materialization, err = arrivalTimetable.NewMaterialization(arrivalTimetable.MaterializationDependencies{
			GroupRepo: boundRepos.ActivityGroup, ScheduleRepo: boundRepos.ActivitySchedule,
			EnrollmentRepo: boundRepos.StudentEnrollment, SupervisorRepo: boundRepos.ActivitySupervisor,
			PeriodRepo: boundRepos.CalendarPeriod, InstanceRepo: activityInstanceRepo, StaffRepo: instanceStaff,
			StudentRepo: boundRepos.InstanceStudent, ExceptionRepo: timetablesqltest.NewActivityExceptionRepository(db),
			TimeframeRepo: boundRepos.Timeframe, CareBounds: boundRepos.Student, RecurrenceLock: lock, DB: db,
		})
		if err != nil {
			panic(err)
		}
	}
	templateWrites, err := arrivalTimetable.NewTemplateAdministration(arrivalTimetable.TemplateAdministrationDependencies{
		Groups:               boundRepos.ActivityGroup,
		Categories:           boundRepos.ActivityCategory,
		Schedules:            boundRepos.ActivitySchedule,
		Enrollments:          boundRepos.StudentEnrollment,
		Supervisors:          boundRepos.ActivitySupervisor,
		Instances:            activityInstanceRepo,
		InstanceStaff:        instanceStaff,
		Participants:         boundRepos.InstanceStudent,
		Timeframes:           boundRepos.Timeframe,
		EducationGroups:      educationRepo.NewGroupRepository(db),
		Materialization:      materialization,
		Deviations:           testSeriesDeviations(options.instances),
		CareOfferings:        testCareOfferingChecks(options),
		ResyncOfferingRoster: options.resyncOfferingRoster,
		RecurrenceLock:       lock,
		SchoolClasses: arrivalTimetable.SchoolClassRules{
			MinGradeLevel: schoolclass.MinGradeLevel, MaxGradeLevel: schoolclass.MaxGradeLevel, Normalize: schoolclass.Normalize,
		},
		DB:    db,
		Today: today,
	})
	if err != nil {
		panic(err)
	}
	corrections, err := arrivalTimetable.NewAttendanceCorrections(arrivalTimetable.AttendanceCorrectionDependencies{
		Participants: boundRepos.InstanceStudent,
		Instances:    activityInstanceRepo,
		Trail:        repositories.TimetableAttendanceCorrectionTrail(auditRepo.NewAttendanceCorrectionRepository(auditRepo.NewRuntime(db, auditModels.TenantIDFromContext))),
		People:       usersRepo.NewPersonRepository(db),
		Locks:        recovery,
	})
	if err != nil {
		panic(err)
	}
	return &testTimetable{
		TemplateAdministration: templateWrites,
		data:                   timetableData,
		conflicts:              conflicts,
		lock:                   lock,
		corrections:            corrections,
	}
}

// testCareOfferingChecks binds the suite's care-offering callbacks; an
// omitted callback accepts, and an invalid linked offering is the conflict,
// as the composition root binds Enrollment.
func testCareOfferingChecks(options testTimetableOptions) arrivalTimetable.CareOfferingChecks {
	checks := arrivalTimetable.CareOfferingChecks{
		ValidateSeries:         options.validateCareOfferingSeries,
		ValidateOfferingSource: options.validateOfferingSource,
		IsConflict: func(err error) bool {
			return errors.Is(err, enrollmentModels.ErrCareOfferingInvalid)
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

// testSeriesDeviations binds the split's deviation port to the suite's
// instance lifecycle, or to one that preserves nothing.
func testSeriesDeviations(instances timetableplanning.InstanceService) arrivalTimetable.SeriesDeviations {
	if instances == nil {
		return noSeriesDeviations{}
	}
	preserver, err := timetableplanning.NewSeriesDeviationPreserver(instances)
	if err != nil {
		panic(err)
	}
	return lifecycleSeriesDeviations{preserver: preserver}
}

type lifecycleSeriesDeviations struct {
	preserver *timetableplanning.SeriesDeviationPreserver
}

func (d lifecycleSeriesDeviations) LockDeviationDays(ctx context.Context, tenantID int64, from, to timezone.Date) error {
	return d.preserver.LockDeviationDays(ctx, tenantID, from, to)
}

func (d lifecycleSeriesDeviations) SnapshotDeviations(ctx context.Context, from, to timezone.Date, templateID int64) (arrivalTimetable.PreservedDeviations, error) {
	snapshot, err := d.preserver.SnapshotDeviations(ctx, from, to, templateID)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

type noSeriesDeviations struct{}

func (noSeriesDeviations) LockDeviationDays(context.Context, int64, timezone.Date, timezone.Date) error {
	return nil
}

func (noSeriesDeviations) SnapshotDeviations(context.Context, timezone.Date, timezone.Date, int64) (arrivalTimetable.PreservedDeviations, error) {
	return noPreservedDeviations{}, nil
}

type noPreservedDeviations struct{}

func (noPreservedDeviations) Count() int { return 0 }

func (noPreservedDeviations) Reapply(context.Context, int64, *int64) (int, error) { return 0, nil }

// testArrivalBaselines serves the conflict detection's arrival port from Care
// Plan's baseline projection, the way the composition root does.
type testArrivalBaselines struct {
	reader careplan.ArrivalBaselineReader
}

func (b testArrivalBaselines) ProjectArrivals(ctx context.Context, studentIDs []int64, from, to timezone.Date) (arrivalTimetable.ArrivalBaselines, error) {
	projection, err := b.reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return testArrivalProjection{projection: projection}, nil
}

type testArrivalProjection struct {
	projection *careplan.ArrivalBaselineProjection
}

func (p testArrivalProjection) ExpectedArrival(studentID int64, date timezone.Date) (time.Time, bool) {
	row := p.projection.ForDate(studentID, date)
	if row == nil {
		return time.Time{}, false
	}
	return row.ExpectedArrival, true
}

func mustTimetableTestRepositories(db *bun.DB, clocks ...func() time.Time) repositories.TimetableTestRepositories {
	repos, err := repositories.NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		panic(err)
	}
	return repos
}

// calendarPeriodUsageFor serves the usage port from the test repository
// factory's planning-owner read, converting by shape.
func calendarPeriodUsageFor(repos repositories.TimetableTestRepositories) CalendarPeriodUsage {
	return usageFunc(func(ctx context.Context) (map[int64]CalendarPeriodUsageCounts, error) {
		values, err := repos.CalendarPeriodUsage().Usage(ctx)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]CalendarPeriodUsageCounts, len(values))
		for id, value := range values {
			result[id] = CalendarPeriodUsageCounts(value)
		}
		return result, nil
	})
}

type usageFunc func(context.Context) (map[int64]CalendarPeriodUsageCounts, error)

func (f usageFunc) UsageCounts(ctx context.Context) (map[int64]CalendarPeriodUsageCounts, error) {
	return f(ctx)
}
