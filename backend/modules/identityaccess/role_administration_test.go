package identityaccess_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/require"
)

const policyTenant int64 = 42

func tenantOf(id int64) *int64 { return &id }

// The guardian and legacy-teacher blocks key off what a role actually grants,
// not off its label: role names are lowercased on write, so a school role
// called "Guardian" or "Teacher" collides with the platform names.
func TestValidateAssignableSchoolRoleDecidesByTierNotName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role *identityaccess.RoleFacts
		want error
	}{
		{"platform guardian role stays blocked", systemRole(identityaccess.BaseRoleGuardian), identityaccess.ErrRoleGuardianNotAssignable},
		{"custom role with guardian base role stays blocked",
			&identityaccess.RoleFacts{Name: "elternvertretung", TenantID: tenantOf(policyTenant), BaseRole: ptr(identityaccess.BaseRoleGuardian)},
			identityaccess.ErrRoleGuardianNotAssignable},
		{"tenant role merely named guardian is assignable",
			&identityaccess.RoleFacts{Name: identityaccess.BaseRoleGuardian, TenantID: tenantOf(policyTenant), BaseRole: ptr(identityaccess.BaseRoleUser)},
			nil},
		{"legacy system teacher role stays blocked", systemRole(" Teacher "), identityaccess.ErrRoleLegacyTeacherNotAssignable},
		{"tenant role named teacher is assignable",
			&identityaccess.RoleFacts{Name: "teacher", TenantID: tenantOf(policyTenant), BaseRole: ptr(identityaccess.BaseRoleUser)},
			nil},
		{"role of a different school is rejected",
			&identityaccess.RoleFacts{Name: "betreuung", TenantID: tenantOf(policyTenant + 1), BaseRole: ptr(identityaccess.BaseRoleUser)},
			identityaccess.ErrRoleForeignTenant},
		{"platform user role is assignable", systemRole(identityaccess.BaseRoleUser), nil},
		{"platform lehrkraft role is assignable", systemRole("lehrkraft"), nil},
		// tenant_id NULL without is_system is not a platform role but a broken
		// row; refuse rather than treat it as globally assignable.
		{"global role that is not a system role",
			&identityaccess.RoleFacts{Name: "irgendwas", BaseRole: ptr(identityaccess.BaseRoleUser)},
			identityaccess.ErrRoleNotAssignable},
		{"missing role", nil, identityaccess.ErrRoleNotAssignable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := identityaccess.ValidateAssignableSchoolRole(tt.role, policyTenant)
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			require.Same(t, tt.want, err)
		})
	}
}

func TestIsGuardianTierRole(t *testing.T) {
	t.Parallel()

	require.True(t, identityaccess.IsGuardianTierRole(systemRole("guardian")))
	require.True(t, identityaccess.IsGuardianTierRole(customRole("eltern", ptr(" Guardian "))))
	require.False(t, identityaccess.IsGuardianTierRole(customRole("guardian", ptr(identityaccess.BaseRoleUser))))
	require.False(t, identityaccess.IsGuardianTierRole(customRole("guardian", nil)))
	require.False(t, identityaccess.IsGuardianTierRole(nil))
}

func TestIsSchoolIdentityRequestError(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{
		identityaccess.ErrSchoolIdentityNamesRequired,
		identityaccess.ErrSchoolIdentityPersonIsStudent,
		identityaccess.ErrSchoolIdentityTagUnknown,
		identityaccess.ErrSchoolIdentityTagConflict,
		identityaccess.ErrSchoolIdentityTagTaken,
	} {
		require.True(t, identityaccess.IsSchoolIdentityRequestError(sentinel))
		require.True(t, identityaccess.IsSchoolIdentityRequestError(fmt.Errorf("provision: %w", sentinel)))
	}
	require.False(t, identityaccess.IsSchoolIdentityRequestError(errors.New("database is down")))
	require.False(t, identityaccess.IsSchoolIdentityRequestError(identityaccess.ErrRoleLehrkraftCaregiverProfile))
	require.False(t, identityaccess.IsSchoolIdentityRequestError(nil))
}

func TestPermissionFullName(t *testing.T) {
	t.Parallel()

	permission := identityaccess.Permission{Name: "groups_read", Resource: "groups", Action: "read"}
	require.Equal(t, "groups:read", permission.FullName())
}

func TestAssignableSchoolRoleAuthorizationGrantData(t *testing.T) {
	t.Parallel()

	var missing *identityaccess.AssignableSchoolRole
	present, _, _, _, _, permissions := missing.AuthorizationGrantData()
	require.False(t, present)
	require.Nil(t, permissions)

	role := &identityaccess.AssignableSchoolRole{
		Role:        identityaccess.Role{Name: "ogs", TenantID: tenantOf(policyTenant), BaseRole: ptr(identityaccess.BaseRoleUser)},
		Permissions: []string{"groups:read"},
	}
	present, name, baseRole, system, tenantBound, permissions := role.AuthorizationGrantData()
	require.True(t, present)
	require.Equal(t, "ogs", name)
	require.Equal(t, identityaccess.BaseRoleUser, *baseRole)
	require.False(t, system)
	require.True(t, tenantBound)
	require.Equal(t, []string{"groups:read"}, permissions)

	// The grant check gets a copy; mutating it leaves the role untouched.
	permissions[0] = "admin:*"
	require.Equal(t, []string{"groups:read"}, role.Permissions)
}

// The texts are the wire contract the RBAC routes render.
func TestRoleAdministrationSentinelTexts(t *testing.T) {
	t.Parallel()

	require.EqualError(t, identityaccess.ErrSystemRoleImmutable, "system roles cannot be modified")
	require.EqualError(t, identityaccess.ErrPermissionNotFound, "permission not found")
	require.EqualError(t, identityaccess.ErrRoleNotFound, "role not found")
	require.EqualError(t, identityaccess.ErrRoleNotAssignable, "Die angegebene Rolle existiert nicht")
	require.EqualError(t, identityaccess.ErrLehrkraftRoleImmutable, "Ein Lehrkraft-Konto kann nicht umgestellt werden")
}
