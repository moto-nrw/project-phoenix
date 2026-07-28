package auth

import (
	"context"
	"errors"
	"strings"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// Which roles may be handed out when an account is attached to a school. Shared
// by every path that grants school access — operator provisioning, the
// operator-led school-access management (issue #1021) and the tenant-side
// /auth/link-to-tenant — so the rules cannot drift apart per entry point.
var (
	// ErrRoleNotAssignable is returned when the requested role does not exist
	// (or is not a role that may be handed out for a school at all).
	ErrRoleNotAssignable = errors.New("Die angegebene Rolle existiert nicht") //nolint:staticcheck // ST1005: user-facing German message

	// ErrRoleForeignTenant is returned when a tenant-scoped role belongs to a
	// different school than the one being granted.
	ErrRoleForeignTenant = errors.New("Diese Rolle existiert an der Zielschule nicht") //nolint:staticcheck // ST1005: user-facing German message

	// ErrRoleGuardianNotAssignable is returned for the guardian role, which is
	// granted exclusively through the guardian invitation flow.
	ErrRoleGuardianNotAssignable = errors.New("Sorgeberechtigten-Zugänge werden über den Einladungs-Flow für Sorgeberechtigte vergeben") //nolint:staticcheck // ST1005: user-facing German message

	// ErrRoleLegacyTeacherNotAssignable is returned for the retired teacher
	// role; caregiver accounts use the user role instead.
	ErrRoleLegacyTeacherNotAssignable = errors.New("Die alte Rolle 'teacher' wird nicht mehr vergeben; bitte die Rolle 'user' verwenden") //nolint:staticcheck // ST1005: user-facing German message

	// ErrRoleGrantNotPermitted is returned when the role exists and is
	// assignable at this school, but the acting account is not allowed to hand
	// it out — granting an admin-tier role requires users:manage.
	ErrRoleGrantNotPermitted = errors.New("Du darfst diese Rolle nicht vergeben") //nolint:staticcheck // ST1005: user-facing German message
)

// ValidateAssignableSchoolRole resolves a role and rejects it when it must not
// be assigned for the given school.
//
// A role qualifies when it is either a platform system role (tenant_id NULL) or
// a custom role of that very school. Guardian and the legacy teacher role are
// never assignable through school-access management.
func ValidateAssignableSchoolRole(
	ctx context.Context,
	repo authModels.RoleRepository,
	roleID, tenantID int64,
) (*authModels.Role, error) {
	if roleID <= 0 {
		return nil, ErrRoleNotAssignable
	}
	role, err := repo.FindByID(ctx, roleID)
	if err != nil || role == nil {
		return nil, ErrRoleNotAssignable
	}
	if role.TenantID != nil && *role.TenantID != tenantID {
		return nil, ErrRoleForeignTenant
	}
	if role.TenantID == nil && !role.IsSystem {
		return nil, ErrRoleNotAssignable
	}
	switch strings.ToLower(strings.TrimSpace(role.Name)) {
	case authModels.BaseRoleGuardian:
		return nil, ErrRoleGuardianNotAssignable
	case "teacher":
		return nil, ErrRoleLegacyTeacherNotAssignable
	}
	return role, nil
}
