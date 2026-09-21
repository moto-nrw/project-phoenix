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
	supervisorRepo := presenceCompose.NewLegacyGroupSupervisorRepository(repositories.NewPresenceSupervisionRecords(db))
	var today func() timezone.Date
	if len(clocks) > 0 && clocks[0] != nil {
		clock := clocks[0]
		today = func() timezone.Date { return timezone.DateFromTime(clock()) }
		activityInstanceRepo = timetablesqltest.NewActivityInstanceRepository(db, clock)
		supervisorRepo = presenceCompose.NewLegacyGroupSupervisorRepository(repositories.NewPresenceSupervisionRecords(db), clock)
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
	deps := timetableplanning.TimetableDataDependencies{
		InstanceStudentRepo:        boundRepos.InstanceStudent,
		ActivityInstanceRepo:       activityInstanceRepo,
		ActivityExceptionRepo:      timetablesqltest.NewActivityExceptionRepository(db),
		ActivityScheduleRepo:       boundRepos.ActivitySchedule,
		InstanceStaffRepo:          timetablesqltest.NewInstanceStaffRepository(db),
		StaffShiftRepo:             boundRepos.StaffShift,
		StaffRepo:                  boundRepos.Staff,
		CalendarPeriodRepo:         boundRepos.CalendarPeriod,
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
		RecoveryRepo:               repositories.NewActivityRecoveryRepository(db, boundRepos.InstanceStudent),
		DB:                         db,
		Today:                      today,
	}
	return timetableplanning.NewTimetableDataService(deps)
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
