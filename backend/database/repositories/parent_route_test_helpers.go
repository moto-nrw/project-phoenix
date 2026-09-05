package repositories

import (
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
	"github.com/uptrace/bun"
)

type ParentRouteTestRepositories struct {
	ParentChild            parentModels.ChildRepository
	Student                usersModels.StudentRepository
	Person                 usersModels.PersonRepository
	GuardianProfile        usersModels.GuardianProfileRepository
	StudentGuardian        usersModels.StudentGuardianRepository
	StudentStatusDay       activeModels.StudentStatusDayOverviewRepository
	StudentPickupException scheduleModels.StudentPickupExceptionRepository
	ExcusedAbsenceRequest  activeModels.ExcusedAbsenceRequestRepository
}

func NewParentRouteTestRepositories(db *bun.DB) (ParentRouteTestRepositories, error) {
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	slots := timetableInstanceStudentRepository{timetable: NewUnobservedTimetable(db)}
	care, err := NewCarePlan(db, people, slots)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	r := &Factory{db: db,
		ParentChild: parentRepo.NewChildRepository(carePlanLegacy.NewParentRuntime(db)),
		Student:     usersRepo.NewStudentRepository(db), Person: usersRepo.NewPersonRepository(db),
		GuardianProfile: usersRepo.NewGuardianProfileRepository(db), StudentGuardian: usersRepo.NewStudentGuardianRepository(db),
	}
	r.BindPeopleDirectory(people)
	r.bindCarePlanAdapters(care)
	return ParentRouteTestRepositories{
		ParentChild: r.ParentChild, Student: r.Student, Person: r.Person,
		GuardianProfile: r.GuardianProfile, StudentGuardian: r.StudentGuardian,
		StudentStatusDay: r.StudentStatusDay, StudentPickupException: r.StudentPickupException, ExcusedAbsenceRequest: r.ExcusedAbsenceRequest,
	}, nil
}
