package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	binding "github.com/moto-nrw/project-phoenix/modules/dataimport/internal/adapters/runtimebinding"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// NewSchoolRolePolicy binds the public facts to the Identity & Access
// decisions. It does not look up the role again or duplicate grant or
// provisioning policy.
func NewSchoolRolePolicy() dataimport.SchoolRolePolicy { return schoolRolePolicy{} }

type schoolRolePolicy struct{}

func (schoolRolePolicy) Validate(role *dataimport.SchoolRole, tenantID int64) error {
	return identityaccess.ValidateAssignableSchoolRole(assignmentFacts(role), tenantID)
}

func (schoolRolePolicy) NeedsCaregiver(role *dataimport.SchoolRole) bool {
	return identityaccess.RoleNeedsCaregiverProfile(assignmentFacts(role))
}

// These policies consume only classification and tenancy facts, not loaded
// permission relations. Grant checks consume SchoolRole directly elsewhere.
func assignmentFacts(role *dataimport.SchoolRole) *identityaccess.RoleFacts {
	return binding.AssignmentFacts(role, newAssignmentFacts)
}

func newAssignmentFacts(id int64, school *int64, name string, system bool, base *string) identityaccess.RoleFacts {
	return identityaccess.RoleFacts{ID: id, TenantID: school, Name: name, IsSystem: system, BaseRole: base}
}
