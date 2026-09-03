package timetable

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
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
	// Template rows resolve their education group name through the School
	// Structure owner, so the activity group repository must come from the
	// bound factory, exactly as in production composition.
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		panic(err)
	}
	boundRepos := repositories.NewFactory(db)
	boundRepos.BindSchoolStructure(groups)
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
		InstanceStudentRepo:   repositories.NewFactory(db).InstanceStudent,
		ActivityInstanceRepo:  activityInstanceRepo,
		ActivityExceptionRepo: scheduleRepo.NewActivityExceptionRepository(db),
		ActivityScheduleRepo:  activitiesRepo.NewScheduleRepository(db),
		InstanceStaffRepo:     scheduleRepo.NewInstanceStaffRepository(db),
		StaffShiftRepo:        scheduleRepo.NewStaffShiftRepository(db),
		StaffRepo:             repositories.NewFactory(db).Staff,
		CalendarPeriodRepo:    scheduleRepo.NewCalendarPeriodRepository(db),
		ActiveGroupRepo:       repositories.NewFactory(db).ActiveGroup,
		SupervisorRepo:        supervisorRepo,
		ArrivalScheduleRepo:   scheduleRepo.NewStudentArrivalScheduleRepository(db),
		ArrivalBaselines: scheduleSvc.NewArrivalBaselineService(
			scheduleRepo.NewStudentArrivalScheduleRepository(db),
			usersRepo.NewStudentRepository(db),
			educationRepo.NewClassArrivalTimeRepository(db),
			scheduleRepo.NewClassArrivalExceptionRepository(db),
			repositories.NewFactory(db).RequestChildOffering,
			enrollmentRepo.NewCareOfferingRepository(db),
			nil,
		),
		ArrivalExceptionRepo: scheduleRepo.NewStudentArrivalExceptionRepository(db),
		PickupScheduleRepo:   scheduleRepo.NewStudentPickupScheduleRepository(db),
		PickupBaselines: scheduletest.NewPickupBaselineService(
			scheduleRepo.NewStudentPickupScheduleRepository(db),
			repositories.NewFactory(db).RequestChildOffering,
			enrollmentRepo.NewCareOfferingRepository(db),
		),
		PickupExceptionRepo:        scheduleRepo.NewStudentPickupExceptionRepository(db),
		VisitRepo:                  activeRepo.NewVisitRepository(db),
		RoomRepo:                   boundRepos.Room,
		ActivityCategoryRepo:       activitiesRepo.NewCategoryRepository(db),
		ActivityGroupRepo:          boundRepos.ActivityGroup,
		ActivitySupervisorRepo:     activitiesRepo.NewSupervisorPlannedRepository(db),
		StudentEnrollmentRepo:      repositories.NewFactory(db).StudentEnrollment,
		TimeframeRepo:              scheduleRepo.NewTimeframeRepository(db),
		EducationGroupRepo:         educationRepo.NewGroupRepository(db),
		ValidateCareOfferingSeries: validateCareOfferingSeries,
		ValidateOfferingSource:     validateOfferingSource,
		ResyncOfferingRoster:       resyncOfferingRoster,
		DeviationEventRepo:         auditRepo.NewDeviationEventRepository(auditRepo.NewRuntime(db, auditModels.TenantIDFromContext)),
		ConflictAckRepo:            scheduleRepo.NewTimetableConflictAckRepository(db),
		RecoveryRepo:               scheduleRepo.NewActivityRecoveryRepository(db),
		DB:                         db,
		Today:                      today,
	}
	return scheduleSvc.NewTimetableDataService(deps)
}
