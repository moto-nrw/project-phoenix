//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type ParentRouteTestRepositories struct {
	// ParentPortalRepositories carries the guardian child repository
	// (ParentChild) without the school names.
	carePlanCompose.ParentPortalRepositories
	StudentStatusDay      *StudentStatusDayRepository
	CareExceptions        careplan.Capability
	ExcusedAbsenceRequest *ExcusedAbsenceRequestRepository
	// ExcusedRequests is the Care Plan excused-absence workflow over the same
	// graph, scoped school-wide because parent routes never decide requests.
	ExcusedRequests careplan.ExcusedAbsenceRequests
}

// ParentChildLinks lists the children a guardian account reaches, as the
// parent calendar reads them.
func (r ParentRouteTestRepositories) ParentChildLinks() carePlanCompose.ParentChildLinks {
	return carePlanCompose.NewParentChildLinks(r.ParentChild)
}

// schoolWideReviewScope is the review scope of a caller who may decide every
// child's request, for the test graphs that do not test the production
// policy. Care Plan's own suites take the same scope from careplantest.
func schoolWideReviewScope(context.Context) (carePlanCompose.ReviewScope, error) {
	return carePlanCompose.ReviewScope{SchoolWide: true}, nil
}

func NewParentRouteTestRepositories(db transactionruntime.Database, members MembershipTestRepositories) (ParentRouteTestRepositories, error) {
	people := members.People
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	bookings, err := NewUnobservedTimetable(db, people, rooms)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	slots := timetableCompose.NewInstanceStudentRepository(MustPresenceReads(db), bookings, NewUnloggedStudentPresence(db), NewPartialAbsencePreview(db))
	care, err := NewCarePlan(db, people, slots)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	excusedRequests, err := NewExcusedAbsenceRequests(ExcusedRequestWiring{
		CarePlan: care, Students: members.Student, Persons: members.Person, Scope: schoolWideReviewScope,
	})
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	return ParentRouteTestRepositories{
		ParentPortalRepositories: carePlanCompose.ParentPortalRepositories{
			ParentChild: carePlanCompose.NewParentChildRepository(NewPeopleIdentityAccess(db, nil).ListActiveAccountSchoolIDs, ParentPortalDirectory{People: people}),
		},
		StudentStatusDay: NewStudentStatusDayRepository(care), CareExceptions: care, ExcusedAbsenceRequest: NewExcusedAbsenceRequestRepository(care),
		ExcusedRequests: excusedRequests,
	}, nil
}
