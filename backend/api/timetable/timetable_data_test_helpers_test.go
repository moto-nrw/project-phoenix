package timetable

import (
	"context"

	"github.com/uptrace/bun"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	facilitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
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
	deps := scheduleSvc.TimetableDataDependencies{
		InstanceStudentRepo:        scheduleRepo.NewInstanceStudentRepository(db),
		ActivityInstanceRepo:       scheduleRepo.NewActivityInstanceRepository(db),
		ActivityExceptionRepo:      scheduleRepo.NewActivityExceptionRepository(db),
		ActivityScheduleRepo:       activitiesRepo.NewScheduleRepository(db),
		InstanceStaffRepo:          scheduleRepo.NewInstanceStaffRepository(db),
		ActiveGroupRepo:            activeRepo.NewGroupRepository(db),
		SupervisorRepo:             activeRepo.NewGroupSupervisorRepository(db),
		ArrivalScheduleRepo:        scheduleRepo.NewStudentArrivalScheduleRepository(db),
		ArrivalExceptionRepo:       scheduleRepo.NewStudentArrivalExceptionRepository(db),
		PickupScheduleRepo:         scheduleRepo.NewStudentPickupScheduleRepository(db),
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
		DB:                         db,
	}
	return scheduleSvc.NewTimetableDataService(deps)
}
