package repositories

import (
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

// NewAuthTestParentEnrollments supplies the parent enrollment requests the
// auth constructors read, with the school names bound. The People Directory
// and School Membership repositories of the auth graph are composed by
// services (#2743); the auth events and the operator ledger come from the
// Audit composition.
func NewAuthTestParentEnrollments(db *bun.DB) (parentModels.EnrollmentRequestRepository, error) {
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	r := &Factory{
		db:                      db,
		ParentEnrollmentRequest: parentRepo.NewEnrollmentRequestRepository(parentRepo.RuntimeFunc(carePlanCompose.TenantAmbientDatabase(db)), enrollmentCompose.New(), identityAccountDirectory{accounts: newIdentityAccess(db, nil)}),
	}
	r.BindOrganizationTenancy(organizations)
	return r.ParentEnrollmentRequest, nil
}
