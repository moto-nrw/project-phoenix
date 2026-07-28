package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// legacyTeacherRoleName is the retired system role that caregiver accounts used
// before they moved to the user role.
const legacyTeacherRoleName = "teacher"

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
	if err != nil {
		if !isNotFoundError(err) {
			// A lookup that failed for any other reason (DB unreachable, RLS)
			// is not a verdict on the role. Pass it through so callers log it
			// and answer 500, instead of telling the user the role is gone.
			return nil, err
		}
		return nil, ErrRoleNotAssignable
	}
	if role == nil {
		return nil, ErrRoleNotAssignable
	}
	if role.TenantID != nil && *role.TenantID != tenantID {
		return nil, ErrRoleForeignTenant
	}
	if role.TenantID == nil && !role.IsSystem {
		return nil, ErrRoleNotAssignable
	}
	// Guardian is decided by tier, not by name. Role.Validate lowercases every
	// name on write, so a school creating a role labelled "Guardian" would land
	// on the same string as the platform role and lock itself out of it. What
	// actually belongs in the guardian invitation flow is a role that hands out
	// guardian privileges, which is exactly what the base role records.
	if authorize.EffectiveBaseRole(role) == authModels.BaseRoleGuardian {
		return nil, ErrRoleGuardianNotAssignable
	}

	// The retired teacher role has no base role to go by — it predates the
	// column and was never backfilled — so it stays a name match, narrowed to
	// system roles. A school's own custom "teacher" role is a different role
	// that happens to share the label and remains assignable.
	if role.IsSystem && strings.EqualFold(strings.TrimSpace(role.Name), legacyTeacherRoleName) {
		return nil, ErrRoleLegacyTeacherNotAssignable
	}

	return role, nil
}
