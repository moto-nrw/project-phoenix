package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
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
	if role == nil {
		return nil
	}
	return &identityaccess.RoleFacts{
		ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
	}
}
