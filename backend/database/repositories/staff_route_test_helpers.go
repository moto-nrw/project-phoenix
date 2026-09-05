package repositories

import (
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	workforceRepo "github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type AbsenceTypeTestRepositories struct {
	Types      activeModels.StaffAbsenceTypeRepository
	Allowances activeModels.StaffAbsenceTypeAllowanceRepository
	Changes    activeModels.StaffAbsenceTypeAllowanceChangeRepository
	Absences   activeModels.StaffAbsenceRepository
}

func NewAbsenceTypeTestRepositories(db *bun.DB) AbsenceTypeTestRepositories {
	return AbsenceTypeTestRepositories{
		Types:      activeRepo.NewStaffAbsenceTypeRepository(db),
		Allowances: workforceRepo.NewStaffAbsenceTypeAllowanceRepository(db),
		Changes:    workforceRepo.NewStaffAbsenceTypeAllowanceChangeRepository(db),
		Absences:   activeRepo.NewStaffAbsenceRepository(db),
	}
}

type ShiftTypeTestRepositories struct {
	Types      scheduleModels.ShiftTypeRepository
	Categories activitiesModels.CategoryRepository
}

func NewShiftTypeTestRepositories(db *bun.DB) ShiftTypeTestRepositories {
	return ShiftTypeTestRepositories{Types: scheduleRepo.NewShiftTypeRepository(db), Categories: activitiesRepo.NewCategoryRepository(db)}
}

func NewStudentLookupTestRepository(db *bun.DB) usersModels.StudentRepository {
	return usersRepo.NewStudentRepository(db)
}

func NewGuardianProfileTestRepository(db *bun.DB) usersModels.GuardianProfileRepository {
	return usersRepo.NewGuardianProfileRepository(db)
}

func NewNotificationPreferenceTestRepository(db *bun.DB) usersModels.NotificationPreferenceRepository {
	return usersRepo.NewNotificationPreferenceRepository(db)
}
