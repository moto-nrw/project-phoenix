package repositories

import (
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

// NewAuthTestRepositories supplies the compatibility carrier required by the
// auth constructors. Only auth, invitation and identity dependencies are built;
// API tests receive the resulting auth capabilities, never this carrier. The
// auth events and the operator ledger come from the Audit composition.
func NewAuthTestRepositories(db *bun.DB) (*Factory, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	r := &Factory{
		db: db, Person: members.Person,
		Staff: members.Staff, Teacher: members.Teacher, GroupTeacher: members.GroupTeacher, ClassTeacher: members.ClassTeacher,
		RFIDCard: newIdentityAccess(db, nil), Student: NewStudentRepository(db),
		GuardianProfile: NewGuardianProfileRepository(db), StudentGuardian: NewStudentGuardianRepository(db),
		ParentEnrollmentRequest: parentRepo.NewEnrollmentRequestRepository(parentRepo.RuntimeFunc(carePlanCompose.TenantAmbientDatabase(db)), enrollmentCompose.New(), identityAccountDirectory{accounts: newIdentityAccess(db, nil)}),
	}
	r.BindOrganizationTenancy(organizations)
	return r, nil
}
