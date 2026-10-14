package services

import (
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

// NewAuthTestParentPortal supplies the parent enrollment requests the auth
// constructors read (ParentEnrollmentRequest), with the school names bound.
// The People Directory and School Membership repositories of the auth graph
// are composed by services (#2743); the auth events and the operator ledger
// come from the Audit composition.
func NewAuthTestParentPortal(db *bun.DB) (carePlanCompose.ParentPortalRepositories, error) {
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return carePlanCompose.ParentPortalRepositories{}, err
	}
	requests := carePlanCompose.NewParentEnrollmentRequestRepository(db, enrollmentCompose.New(), IdentityAccountDirectory{Accounts: NewPeopleIdentityAccess(db, nil)})
	return carePlanCompose.ParentPortalRepositories{ParentEnrollmentRequest: requests}.WithSchools(ParentPortalSchools{Schools: organizations}), nil
}
