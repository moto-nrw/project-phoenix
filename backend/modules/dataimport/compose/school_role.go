package compose

import (
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// NewSchoolRolePolicy binds the public facts to the existing owner decisions.
// It does not look up the role again or duplicate grant/provisioning policy.
func NewSchoolRolePolicy() dataimport.SchoolRolePolicy { return schoolRolePolicy{} }

type schoolRolePolicy struct{}

func (schoolRolePolicy) Validate(role *dataimport.SchoolRole, tenantID int64) error {
	_, err := authService.ValidateResolvedAssignableSchoolRole(assignmentFacts(role), tenantID)
	return err
}

func (schoolRolePolicy) NeedsCaregiver(role *dataimport.SchoolRole) bool {
	if role == nil {
		return false
	}
	return identityaccess.RoleNeedsCaregiverProfile(&identityaccess.RoleFacts{
		ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
	})
}

// These policies consume only classification and tenancy facts, not loaded
// permission relations. Grant checks consume SchoolRole directly elsewhere.
func assignmentFacts(role *dataimport.SchoolRole) *authModels.Role {
	if role == nil {
		return nil
	}
	facts := &authModels.Role{TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole}
	facts.ID = role.ID
	return facts
}
