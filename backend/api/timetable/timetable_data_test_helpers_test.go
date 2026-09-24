package timetable

import (
	"context"
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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetablesqltest"
)

// testTimetableData builds the full TimetableDataService against the test
// database — the test-side equivalent of the factory wiring.
func testTimetableData(db *bun.DB, clocks ...func() time.Time) *timetableplanning.TimetableDataService {
	return testTimetableDataWithCareValidator(db, nil, clocks...)
}

func testTimetableDataWithCareValidator(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	clocks ...func() time.Time,
) *timetableplanning.TimetableDataService {
	return testTimetableDataWithOfferingCallbacks(db, validateCareOfferingSeries, nil, nil, clocks...)
}

func testTimetableDataWithOfferingCallbacks(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	validateOfferingSource func(context.Context, []int64, []int64, *int64) error,
	resyncOfferingRoster func(context.Context, timetableplanning.OfferingRosterResyncInput) error,
	clocks ...func() time.Time,
) *timetableplanning.TimetableDataService {
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
	supervisorRepo := repositories.NewPresenceSessionRecords(db)
	var today func() timezone.Date
	if len(clocks) > 0 && clocks[0] != nil {
		clock := clocks[0]
		today = func() timezone.Date { return timezone.DateFromTime(clock()) }
		activityInstanceRepo = timetablesqltest.NewActivityInstanceRepository(db, clock)
		supervisorRepo = repositories.NewPresenceSessionRecords(db, clock)
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
	deps := timetableplanning.TimetableDataDependencies{
		InstanceStudentRepo:        boundRepos.InstanceStudent,
		ActivityInstanceRepo:       activityInstanceRepo,
		ActivityExceptionRepo:      timetablesqltest.NewActivityExceptionRepository(db),
		ActivityScheduleRepo:       boundRepos.ActivitySchedule,
		InstanceStaffRepo:          timetablesqltest.NewInstanceStaffRepository(db),
		ActiveGroupRepo:            boundRepos.ActiveGroup,
		SupervisorRepo:             supervisorRepo,
		ArrivalBaselines:           arrivalBaselines,
		ArrivalExceptionRepo:       boundRepos.StudentArrivalException,
		PickupScheduleRepo:         boundRepos.StudentPickupSchedule,
		PickupBaselines:            newPickupBaselineService(carePlan, approvedOfferings),
		PickupExceptionRepo:        boundRepos.StudentPickupException,
		Presence:                   presence,
		RoomRepo:                   boundRepos.Room,
		ActivityCategoryRepo:       boundRepos.ActivityCategory,
		ActivityGroupRepo:          boundRepos.ActivityGroup,
		ActivitySupervisorRepo:     boundRepos.ActivitySupervisor,
		StudentEnrollmentRepo:      boundRepos.StudentEnrollment,
		TimeframeRepo:              boundRepos.Timeframe,
		EducationGroupRepo:         educationRepo.NewGroupRepository(db),
		ValidateCareOfferingSeries: validateCareOfferingSeries,
		ValidateOfferingSource:     validateOfferingSource,
		ResyncOfferingRoster:       resyncOfferingRoster,
		DeviationEventRepo:         auditRepo.NewDeviationEventRepository(auditRepo.NewRuntime(db, auditModels.TenantIDFromContext)),
		AttendanceCorrectionRepo:   auditRepo.NewAttendanceCorrectionRepository(auditRepo.NewRuntime(db, auditModels.TenantIDFromContext)),
		PersonRepo:                 usersRepo.NewPersonRepository(db),
		ConflictAcks:               boundRepos.Timetable,
		ConflictDetection:          conflicts,
		RecoveryRepo:               repositories.NewActivityRecoveryRepository(db, boundRepos.InstanceStudent),
		DB:                         db,
		Today:                      today,
	}
	return timetableplanning.NewTimetableDataService(deps)
}

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

type panicTestTB struct{}

func (panicTestTB) Helper()                           {}
func (panicTestTB) Fatalf(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }

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
