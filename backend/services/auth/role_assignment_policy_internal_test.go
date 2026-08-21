package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
)

const policyTestTenantID int64 = 42

func policyTestRole(id int64, name string, tenantID *int64, isSystem bool, baseRole *string) *authModel.Role {
	return &authModel.Role{
		Model:    baseModel.Model{ID: id},
		Name:     name,
		TenantID: tenantID,
		IsSystem: isSystem,
		BaseRole: baseRole,
	}
}

func tenantPtr(id int64) *int64 { return &id }

func baseRolePtr(v string) *string { return &v }

// The guardian and legacy-teacher blocks must key off what a role actually
// grants, not off its label: Role.Validate lowercases every name on write, so a
// school role called "Guardian" or "Teacher" collides with the platform names.
func TestValidateAssignableSchoolRole_TenantRolesAreNotBlockedByName(t *testing.T) {
	t.Parallel()

	otherTenant := policyTestTenantID + 1

	tests := []struct {
		name        string
		role        *authModel.Role
		expectedErr error
	}{
		{
			name:        "platform guardian role stays blocked",
			role:        policyTestRole(1, authModel.BaseRoleGuardian, nil, true, nil),
			expectedErr: ErrRoleGuardianNotAssignable,
		},
		{
			name:        "custom role with guardian base role stays blocked",
			role:        policyTestRole(2, "elternvertretung", tenantPtr(policyTestTenantID), false, baseRolePtr(authModel.BaseRoleGuardian)),
			expectedErr: ErrRoleGuardianNotAssignable,
		},
		{
			name:        "tenant role merely named guardian is assignable",
			role:        policyTestRole(3, authModel.BaseRoleGuardian, tenantPtr(policyTestTenantID), false, baseRolePtr(authModel.BaseRoleUser)),
			expectedErr: nil,
		},
		{
			name:        "legacy system teacher role stays blocked",
			role:        policyTestRole(4, "teacher", nil, true, nil),
			expectedErr: ErrRoleLegacyTeacherNotAssignable,
		},
		{
			name:        "tenant role named teacher is assignable",
			role:        policyTestRole(5, "teacher", tenantPtr(policyTestTenantID), false, baseRolePtr(authModel.BaseRoleUser)),
			expectedErr: nil,
		},
		{
			name:        "role of a different school is rejected",
			role:        policyTestRole(6, "betreuung", tenantPtr(otherTenant), false, baseRolePtr(authModel.BaseRoleUser)),
			expectedErr: ErrRoleForeignTenant,
		},
		{
			name:        "platform user role is assignable",
			role:        policyTestRole(7, authModel.BaseRoleUser, nil, true, nil),
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newStubRoleRepository(tt.role)

			role, err := ValidateAssignableSchoolRole(context.Background(), repo, tt.role.ID, policyTestTenantID)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, role)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, role)
			require.Equal(t, tt.role.ID, role.ID)
		})
	}
}

func TestValidateAssignableSchoolRole_RejectsUnknownAndMalformedRoles(t *testing.T) {
	t.Parallel()

	t.Run("missing role", func(t *testing.T) {
		repo := newStubRoleRepository()

		role, err := ValidateAssignableSchoolRole(context.Background(), repo, 999, policyTestTenantID)

		require.ErrorIs(t, err, ErrRoleNotAssignable)
		require.Nil(t, role)
	})

	t.Run("invalid role id", func(t *testing.T) {
		repo := newStubRoleRepository()

		role, err := ValidateAssignableSchoolRole(context.Background(), repo, 0, policyTestTenantID)

		require.ErrorIs(t, err, ErrRoleNotAssignable)
		require.Nil(t, role)
	})

	// tenant_id NULL without is_system is not a platform role, it is a broken
	// row — refuse rather than treat it as globally assignable.
	t.Run("global role that is not a system role", func(t *testing.T) {
		orphan := policyTestRole(8, "irgendwas", nil, false, baseRolePtr(authModel.BaseRoleUser))
		repo := newStubRoleRepository(orphan)

		role, err := ValidateAssignableSchoolRole(context.Background(), repo, orphan.ID, policyTestTenantID)

		require.ErrorIs(t, err, ErrRoleNotAssignable)
		require.Nil(t, role)
	})
}
