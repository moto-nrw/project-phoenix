//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

type ParentRouteTestRepositories struct {
	// ParentPortalRepositories carries the guardian child repository
	// (ParentChild) without the school names.
	carePlanCompose.ParentPortalRepositories
	StudentStatusDay      *owners.StudentStatusDayRepository
	CareExceptions        careplan.Capability
	ExcusedAbsenceRequest *owners.ExcusedAbsenceRequestRepository
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

func NewParentRouteTestRepositories(db transactionruntime.Database, members owners.MembershipTestRepositories) (ParentRouteTestRepositories, error) {
	people := members.People
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	bookings, err := owners.NewUnobservedTimetable(db, people, rooms)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	slots := timetablePorts.ParticipantRows{Plans: bookings, Presence: owners.NewUnloggedStudentPresence(db), Reads: owners.MustPresenceReads(db)}
	care, err := owners.NewCarePlan(db, people, slots)
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	excusedRequests, err := owners.NewExcusedAbsenceRequests(owners.ExcusedRequestWiring{
		CarePlan: care, Students: members.Student, Persons: members.Person, Scope: schoolWideReviewScope,
	})
	if err != nil {
		return ParentRouteTestRepositories{}, err
	}
	return ParentRouteTestRepositories{
		ParentPortalRepositories: carePlanCompose.ParentPortalRepositories{
			ParentChild: carePlanCompose.NewParentChildRepository(owners.NewPeopleIdentityAccess(db, nil).ListActiveAccountSchoolIDs, owners.ParentPortalDirectory{People: people}),
		},
		StudentStatusDay: owners.NewStudentStatusDayRepository(care), CareExceptions: care, ExcusedAbsenceRequest: owners.NewExcusedAbsenceRequestRepository(care),
		ExcusedRequests: excusedRequests,
	}, nil
}

// NewParentOfferingChangeTestReads binds the parent portal's offering change
// identity read over Care Plan as the production root does.
func NewParentOfferingChangeTestReads(carePlan careplan.Capability) parentportal.OfferingChangeRequestReads {
	return owners.NewParentOfferingChangeReads(enrollmentCompose.NewOfferingChangeRecords(carePlan))
}

// ParentRequestSharingNotFound is the parent portal's answer to a missing or
// foreign shared request, for tests that may not import the workflow.
var ParentRequestSharingNotFound = parentportal.ErrRequestSharingNotFound
