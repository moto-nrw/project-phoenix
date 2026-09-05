package timetable

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
)

// testTimetableData builds the full TimetableDataService against the test
// database — the test-side equivalent of the factory wiring.
func testTimetableData(db *bun.DB, clocks ...func() time.Time) *scheduleSvc.TimetableDataService {
	return testTimetableDataWithCareValidator(db, nil, clocks...)
}

func testTimetableDataWithCareValidator(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	clocks ...func() time.Time,
) *scheduleSvc.TimetableDataService {
	return testTimetableDataWithOfferingCallbacks(db, validateCareOfferingSeries, nil, nil, clocks...)
}

func testTimetableDataWithOfferingCallbacks(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	validateOfferingSource func(context.Context, []int64, []int64, *int64) error,
	resyncOfferingRoster func(context.Context, scheduleSvc.OfferingRosterResyncInput) error,
	clocks ...func() time.Time,
) *scheduleSvc.TimetableDataService {
	boundRepos := mustTimetableTestRepositories(db, clocks...)
	activityInstanceRepo := scheduleRepo.NewActivityInstanceRepository(db)
	supervisorRepo := activeRepo.NewGroupSupervisorRepository(db)
	var today func() timezone.Date
	if len(clocks) > 0 && clocks[0] != nil {
		clock := clocks[0]
		today = func() timezone.Date { return timezone.DateFromTime(clock()) }
		activityInstanceRepo = scheduleRepo.NewActivityInstanceRepository(db, clock)
		supervisorRepo = activeRepo.NewGroupSupervisorRepository(db, clock)
	}
	deps := scheduleSvc.TimetableDataDependencies{
		InstanceStudentRepo:   boundRepos.InstanceStudent,
		ActivityInstanceRepo:  activityInstanceRepo,
		ActivityExceptionRepo: scheduleRepo.NewActivityExceptionRepository(db),
		ActivityScheduleRepo:  boundRepos.ActivitySchedule,
		InstanceStaffRepo:     scheduleRepo.NewInstanceStaffRepository(db),
		StaffShiftRepo:        scheduleRepo.NewStaffShiftRepository(db),
		StaffRepo:             boundRepos.Staff,
		CalendarPeriodRepo:    boundRepos.CalendarPeriod,
		ActiveGroupRepo:       boundRepos.ActiveGroup,
		SupervisorRepo:        supervisorRepo,
		ArrivalScheduleRepo:   boundRepos.StudentArrivalSchedule,
		ArrivalBaselines: scheduleSvc.NewArrivalBaselineService(
			boundRepos.StudentArrivalSchedule,
			usersRepo.NewStudentRepository(db),
			educationRepo.NewClassArrivalTimeRepository(db),
			scheduleRepo.NewClassArrivalExceptionRepository(db),
			boundRepos.RequestChildOffering,
			boundRepos.CareOffering,
			nil,
		),
		ArrivalExceptionRepo: boundRepos.StudentArrivalException,
		PickupScheduleRepo:   boundRepos.StudentPickupSchedule,
		PickupBaselines: scheduletest.NewPickupBaselineService(
			boundRepos.StudentPickupSchedule,
			boundRepos.RequestChildOffering,
			boundRepos.CareOffering,
		),
		PickupExceptionRepo:        boundRepos.StudentPickupException,
		VisitRepo:                  activeRepo.NewVisitRepository(db),
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
		ConflictAckRepo:            scheduleRepo.NewTimetableConflictAckRepository(db),
		RecoveryRepo:               repositories.NewActivityRecoveryRepository(db, boundRepos.InstanceStudent),
		DB:                         db,
		Today:                      today,
	}
	return scheduleSvc.NewTimetableDataService(deps)
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
