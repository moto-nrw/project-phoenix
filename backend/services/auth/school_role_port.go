package auth

import (
	"context"
	"errors"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// The school-role policy belongs to Identity & Access (#3314). The retained
// registration, linking and invitation flows resolve the role themselves and
// ask the policy through their SchoolIdentityProvisioning port; the
// composition root translates the owner's sentinels into the ones below,
// which carry the same messages the HTTP layers render.
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

	// ErrLehrkraftNoCaregiver is returned when an invitation combines the
	// lehrkraft role with caregiver_enabled — the upgrade would grant the
	// full user role and a caregiver profile, defeating the role's
	// class-scoped read-only design (#1772).
	ErrLehrkraftNoCaregiver = errors.New("Die Rolle 'Lehrkraft' kann nicht mit Betreuungsrechten kombiniert werden") //nolint:staticcheck // ST1005: user-facing German message

	// ErrRoleLehrkraftCaregiverProfile rejects linking an account with the
	// Lehrkraft role while its identity at the school still carries a live
	// caregiver profile (users.teachers): the swap would strand the profile
	// and its group supervisions under class_day-only permissions (#1772).
	ErrRoleLehrkraftCaregiverProfile = errors.New("Das Konto hat ein Betreuungsprofil an dieser Schule und kann nicht auf Lehrkraft umgestellt werden") //nolint:staticcheck // ST1005: user-facing German message
)

// validateAssignableSchoolRole resolves a role and refuses it when the school-
// role policy does not allow it for tenantID. A lookup that failed for any
// other reason than a missing row (DB unreachable, RLS) is not a verdict on
// the role and is passed through, so callers log it and answer 500 instead of
// telling the user the role is gone.
func validateAssignableSchoolRole(
	ctx context.Context,
	repo authModels.RoleRepository,
	policy SchoolIdentityProvisioning,
	roleID, tenantID int64,
) (*authModels.Role, error) {
	if roleID <= 0 {
		return nil, ErrRoleNotAssignable
	}
	role, err := repo.FindByID(ctx, roleID)
	if err != nil {
		if !isNotFoundError(err) {
			return nil, err
		}
		return nil, ErrRoleNotAssignable
	}
	if role == nil {
		return nil, ErrRoleNotAssignable
	}
	if policy == nil {
		return nil, ErrAccountLifecycleUnavailable
	}
	if err := policy.ValidateAssignableSchoolRole(RoleFactsOf(role), tenantID); err != nil {
		return nil, err
	}
	return role, nil
}

// isLehrkraftRole reports whether the role is the platform Lehrkraft role.
// Without a composed policy no role is classified as Lehrkraft; the flows
// that must refuse it validate the role through the policy first.
func isLehrkraftRole(policy SchoolIdentityProvisioning, role *authModels.Role) bool {
	return policy != nil && role != nil && policy.IsLehrkraftSystemRole(RoleFactsOf(role))
}
