package repositories

import (
	"context"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/uptrace/bun"
)

// NewStudentGuardianRepository binds relationship permission checks to native
// Identity membership facts for tests that do not construct the serving graph.
func NewStudentGuardianRepository(db *bun.DB) usersModels.StudentGuardianRepository {
	return usersRepo.NewStudentGuardianRepository(db, usersRepo.WithStudentGuardianMemberships(newIdentityAccess(db, nil).FindActiveSchoolMemberships))
}

type ParentRouteTestRepositories struct {
	ParentChild            parentModels.ChildRepository
	Student                usersModels.StudentRepository
	Person                 usersModels.PersonRepository
	GuardianProfile        usersModels.GuardianProfileRepository
	StudentGuardian        usersModels.StudentGuardianRepository
	StudentStatusDay       activeModels.StudentStatusDayOverviewRepository
	StudentPickupException scheduleModels.StudentPickupExceptionRepository
	ExcusedAbsenceRequest  activeModels.ExcusedAbsenceRequestRepository
	// ExcusedRequests is the Care Plan excused-absence workflow over the same
	// graph, scoped school-wide because parent routes never decide requests.
	ExcusedRequests careplan.ExcusedAbsenceRequests
}

// SchoolWideReviewScope is the review scope of a caller who may decide every
// child's request. Test graphs use it where the production policy is not
// under test.
func SchoolWideReviewScope(context.Context) (carePlanCompose.ReviewScope, error) {
	return carePlanCompose.ReviewScope{SchoolWide: true}, nil
}

func NewParentRouteTestRepositories(db *bun.DB) (ParentRouteTestRepositories, error) {
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	slots := timetableInstanceStudentRepository{timetable: NewUnobservedTimetableDependencies(db).Capability}
	care, err := NewCarePlan(db, people, slots)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	r := &Factory{db: db,
		ParentChild: parentRepo.NewChildRepository(newIdentityAccess(db, nil).ListActiveAccountSchoolIDs),
		Student:     NewStudentRepository(db), Person: NewPersonRepository(db),
		GuardianProfile: NewGuardianProfileRepository(db), StudentGuardian: NewStudentGuardianRepository(db),
	}
	r.BindPeopleDirectory(people)
	r.bindCarePlanAdapters(care)
	excusedRequests, err := NewExcusedAbsenceRequests(ExcusedRequestWiring{
		CarePlan: care, Students: r.Student, Persons: r.Person, Scope: SchoolWideReviewScope,
	})
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	return ParentRouteTestRepositories{
		ParentChild: r.ParentChild, Student: r.Student, Person: r.Person,
		GuardianProfile: r.GuardianProfile, StudentGuardian: r.StudentGuardian,
		StudentStatusDay: r.StudentStatusDay, StudentPickupException: r.StudentPickupException, ExcusedAbsenceRequest: r.ExcusedAbsenceRequest,
		ExcusedRequests: excusedRequests,
	}, nil
}
