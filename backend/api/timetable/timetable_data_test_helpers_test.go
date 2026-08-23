package timetable

import (
	"context"

	"github.com/uptrace/bun"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	facilitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
)

// testTimetableData builds the full TimetableDataService against the test
// database — the test-side equivalent of the factory wiring.
func testTimetableData(db *bun.DB) *scheduleSvc.TimetableDataService {
	return testTimetableDataWithCareValidator(db, nil)
}

func testTimetableDataWithCareValidator(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
) *scheduleSvc.TimetableDataService {
	return testTimetableDataWithOfferingCallbacks(db, validateCareOfferingSeries, nil, nil)
}

func testTimetableDataWithOfferingCallbacks(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	validateOfferingSource func(context.Context, []int64, []int64, *int64) error,
	resyncOfferingRoster func(context.Context, scheduleSvc.OfferingRosterResyncInput) error,
) *scheduleSvc.TimetableDataService {
	deps := scheduleSvc.TimetableDataDependencies{
		InstanceStudentRepo:   scheduleRepo.NewInstanceStudentRepository(db),
		ActivityInstanceRepo:  scheduleRepo.NewActivityInstanceRepository(db),
		ActivityExceptionRepo: scheduleRepo.NewActivityExceptionRepository(db),
		ActivityScheduleRepo:  activitiesRepo.NewScheduleRepository(db),
		InstanceStaffRepo:     scheduleRepo.NewInstanceStaffRepository(db),
		StaffShiftRepo:        scheduleRepo.NewStaffShiftRepository(db),
		StaffRepo:             usersRepo.NewStaffRepository(db),
		CalendarPeriodRepo:    scheduleRepo.NewCalendarPeriodRepository(db),
		ActiveGroupRepo:       activeRepo.NewGroupRepository(db),
		SupervisorRepo:        activeRepo.NewGroupSupervisorRepository(db),
		ArrivalScheduleRepo:   scheduleRepo.NewStudentArrivalScheduleRepository(db),
		ArrivalBaselines: scheduleSvc.NewArrivalBaselineService(
			scheduleRepo.NewStudentArrivalScheduleRepository(db),
			usersRepo.NewStudentRepository(db),
			educationRepo.NewClassArrivalTimeRepository(db),
			enrollmentRepo.NewRequestChildOfferingRepository(db),
			enrollmentRepo.NewCareOfferingRepository(db),
			nil,
		),
		ArrivalExceptionRepo: scheduleRepo.NewStudentArrivalExceptionRepository(db),
		PickupScheduleRepo:   scheduleRepo.NewStudentPickupScheduleRepository(db),
		PickupBaselines: scheduletest.NewPickupBaselineService(
			scheduleRepo.NewStudentPickupScheduleRepository(db),
			enrollmentRepo.NewRequestChildOfferingRepository(db),
			enrollmentRepo.NewCareOfferingRepository(db),
		),
		PickupExceptionRepo:        scheduleRepo.NewStudentPickupExceptionRepository(db),
		VisitRepo:                  activeRepo.NewVisitRepository(db),
		RoomRepo:                   facilitiesRepo.NewRoomRepository(db),
		ActivityCategoryRepo:       activitiesRepo.NewCategoryRepository(db),
		ActivityGroupRepo:          activitiesRepo.NewGroupRepository(db),
		ActivitySupervisorRepo:     activitiesRepo.NewSupervisorPlannedRepository(db),
		StudentEnrollmentRepo:      activitiesRepo.NewStudentEnrollmentRepository(db),
		TimeframeRepo:              scheduleRepo.NewTimeframeRepository(db),
		EducationGroupRepo:         educationRepo.NewGroupRepository(db),
		ValidateCareOfferingSeries: validateCareOfferingSeries,
		ValidateOfferingSource:     validateOfferingSource,
		ResyncOfferingRoster:       resyncOfferingRoster,
		DeviationEventRepo:         auditRepo.NewDeviationEventRepository(db),
		ConflictAckRepo:            scheduleRepo.NewTimetableConflictAckRepository(db),
		RecoveryRepo:               scheduleRepo.NewActivityRecoveryRepository(db),
		DB:                         db,
	}
	return scheduleSvc.NewTimetableDataService(deps)
}
