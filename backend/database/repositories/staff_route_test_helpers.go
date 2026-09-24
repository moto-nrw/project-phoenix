package repositories

import (
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/uptrace/bun"
)

func NewAbsenceTypeTestCapability(db *bun.DB) workforce.Capability {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(err)
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		panic(err)
	}
	return workTime
}

type ShiftTypeTestRepositories struct {
	Timetable  timetable.Capability
	Types      *WorkforceShiftTypeRows
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
	return NewStudentRepository(db)
}

func NewGuardianProfileTestRepository(db *bun.DB) usersModels.GuardianProfileRepository {
	return NewGuardianProfileRepository(db)
}
