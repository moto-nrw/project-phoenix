package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

type ParentRouteTestRepositories struct {
	// ParentPortalRepositories carries the guardian child repository
	// (ParentChild) without the school names.
	carePlanCompose.ParentPortalRepositories
	Student               usersModels.StudentRepository
	Person                usersModels.PersonRepository
	GuardianProfile       usersModels.GuardianProfileRepository
	StudentGuardian       usersModels.StudentGuardianRepository
	StudentStatusDay      *services.StudentStatusDayRepository
	CareExceptions        careplan.Capability
	ExcusedAbsenceRequest *services.ExcusedAbsenceRequestRepository
	// ExcusedRequests is the Care Plan excused-absence workflow over the same
	// graph, scoped school-wide because parent routes never decide requests.
	ExcusedRequests careplan.ExcusedAbsenceRequests
}

// ParentChildLinks lists the children a guardian account reaches, as the
// parent calendar reads them.
func (r ParentRouteTestRepositories) ParentChildLinks() carePlanCompose.ParentChildLinks {
	return carePlanCompose.NewParentChildLinks(r.ParentChild)
}

// SchoolWideReviewScope is the review scope of a caller who may decide every
// child's request. Test graphs use it where the production policy is not
// under test.
func SchoolWideReviewScope(context.Context) (carePlanCompose.ReviewScope, error) {
	return carePlanCompose.ReviewScope{SchoolWide: true}, nil
}

func NewParentRouteTestRepositories(db transactionruntime.Database, members services.MembershipTestRepositories) (ParentRouteTestRepositories, error) {
	people := members.People
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	bookings, err := services.NewTimetable(db.Pool(), people, rooms)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	slots := timetableCompose.NewInstanceStudentRepository(db.Pool(), bookings, services.NewUnloggedStudentPresence(db.Pool()), services.NewPartialAbsencePreview(db.Pool()))
	care, err := NewCarePlan(db, people, slots)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	excusedRequests, err := services.NewExcusedAbsenceRequests(services.ExcusedRequestWiring{
		CarePlan: care, Students: members.Student, Persons: members.Person, Scope: SchoolWideReviewScope,
	})
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	return ParentRouteTestRepositories{
		ParentPortalRepositories: carePlanCompose.ParentPortalRepositories{
			ParentChild: carePlanCompose.NewParentChildRepository(services.NewPeopleIdentityAccess(db.Pool(), nil).ListActiveAccountSchoolIDs, services.ParentPortalDirectory{People: people}),
		},
		Student: members.Student, Person: members.Person,
		GuardianProfile: members.GuardianProfile, StudentGuardian: members.StudentGuardian,
		StudentStatusDay: services.NewStudentStatusDayRepository(care), CareExceptions: care, ExcusedAbsenceRequest: services.NewExcusedAbsenceRequestRepository(care),
		ExcusedRequests: excusedRequests,
	}, nil
}
