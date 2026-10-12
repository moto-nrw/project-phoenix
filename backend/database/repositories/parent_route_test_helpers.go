package repositories

import (
	"context"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type ParentRouteTestRepositories struct {
	ParentChild           parentModels.ChildRepository
	Student               usersModels.StudentRepository
	Person                usersModels.PersonRepository
	GuardianProfile       usersModels.GuardianProfileRepository
	StudentGuardian       usersModels.StudentGuardianRepository
	StudentStatusDay      *StudentStatusDayRepository
	CareExceptions        careplan.Capability
	ExcusedAbsenceRequest *ExcusedAbsenceRequestRepository
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

func NewParentRouteTestRepositories(db *bun.DB, members MembershipTestRepositories) (ParentRouteTestRepositories, error) {
	people := members.People
	rooms, err := NewFacilities(db)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	bookings, err := NewTimetable(db, people, rooms)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	slots := timetableCompose.NewInstanceStudentRepository(db, bookings, newStudentPresence(db), NewPartialAbsencePreview(db))
	care, err := NewCarePlan(db, people, slots)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	r := &Factory{db: db,
		ParentChild: parentRepo.NewChildRepository(newIdentityAccess(db, nil).ListActiveAccountSchoolIDs),
	}
	r.bindStudentDirectories(people, people)
	r.bindGuardianDirectories(people)
	r.bindPersonProjections(people)
	r.bindCarePlanAdapters(care)
	excusedRequests, err := NewExcusedAbsenceRequests(ExcusedRequestWiring{
		CarePlan: care, Students: members.Student, Persons: members.Person, Scope: SchoolWideReviewScope,
	})
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	return ParentRouteTestRepositories{
		ParentChild: r.ParentChild, Student: members.Student, Person: members.Person,
		GuardianProfile: members.GuardianProfile, StudentGuardian: members.StudentGuardian,
		StudentStatusDay: r.StudentStatusDay, CareExceptions: care, ExcusedAbsenceRequest: r.ExcusedAbsenceRequest,
		ExcusedRequests: excusedRequests,
	}, nil
}
