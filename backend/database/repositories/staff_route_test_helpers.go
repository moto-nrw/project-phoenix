package repositories

import (
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	workforceRepo "github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
	Timetable  timetable.Capability
	Types      scheduleModels.ShiftTypeRepository
	Categories activitiesModels.CategoryRepository
}

func NewShiftTypeTestRepositories(db *bun.DB) ShiftTypeTestRepositories {
	repos, err := NewTimetableTestRepositories(db)
	if err != nil {
		panic(err)
	}
	return ShiftTypeTestRepositories{Timetable: repos.Timetable, Types: repos.ShiftType, Categories: repos.ActivityCategory}
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
