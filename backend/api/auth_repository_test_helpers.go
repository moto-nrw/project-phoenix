//go:build testhooks

package api

import (
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// NewAuthTestParentPortal supplies the parent enrollment requests the auth
// constructors read (ParentEnrollmentRequest), with the school names bound.
// The People Directory and School Membership repositories of the auth graph
// are composed by services (#2743); the auth events and the operator ledger
// come from the Audit composition.
func NewAuthTestParentPortal(db transactionruntime.Database) (carePlanCompose.ParentPortalRepositories, error) {
	organizations, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		return carePlanCompose.ParentPortalRepositories{}, err
	}
	requests := carePlanCompose.NewParentEnrollmentRequestRepository(db.Pool(), owners.ParentPortalEnrollments{Enrollment: enrollmentCompose.New()}, owners.IdentityAccountDirectory{Accounts: owners.NewPeopleIdentityAccess(db, nil)})
	return carePlanCompose.ParentPortalRepositories{ParentEnrollmentRequest: requests}.WithSchools(owners.ParentPortalSchools{Schools: organizations}), nil
}
