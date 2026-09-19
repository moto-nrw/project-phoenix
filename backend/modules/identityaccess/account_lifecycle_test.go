package identityaccess_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/require"
)

func systemRole(name string) *identityaccess.RoleFacts {
	return &identityaccess.RoleFacts{Name: name, IsSystem: true}
}

func customRole(name string, base *string) *identityaccess.RoleFacts {
	return &identityaccess.RoleFacts{Name: name, BaseRole: base}
}

func ptr(s string) *string { return &s }

// Successor of TestShouldCreateTeacherForRole: the name-based helper became
// the tier-based RoleNeedsCaregiverProfile (#2222). The school's own roles
// are decided by their tier, which the name check could not see at all.
func TestRoleNeedsCaregiverProfile(t *testing.T) {
	t.Parallel()

	require.True(t, identityaccess.RoleNeedsCaregiverProfile(systemRole("teacher")))
	require.True(t, identityaccess.RoleNeedsCaregiverProfile(systemRole("Teacher")))
	require.True(t, identityaccess.RoleNeedsCaregiverProfile(systemRole("user")))
	require.False(t, identityaccess.RoleNeedsCaregiverProfile(systemRole("admin")))
	// A Lehrkraft (#1772) gets the staff record every staff role gets, but
	// deliberately no caregiver profile: it supervises no OGS group.
	require.False(t, identityaccess.RoleNeedsCaregiverProfile(systemRole("lehrkraft")))

	// A school's own role is decided by its tier, not by its label.
	require.True(t, identityaccess.RoleNeedsCaregiverProfile(customRole("OGS-Kraft", ptr(identityaccess.BaseRoleUser))))
	require.False(t, identityaccess.RoleNeedsCaregiverProfile(customRole("OGS-Leitung", ptr(identityaccess.BaseRoleAdmin))))
	// The label alone means nothing: a custom role named "teacher" with an
	// admin tier is an admin role.
	require.False(t, identityaccess.RoleNeedsCaregiverProfile(customRole("teacher", ptr(identityaccess.BaseRoleAdmin))))
	require.False(t, identityaccess.RoleNeedsCaregiverProfile(nil))
}

// The bug of #2222: a school's own role produced a person and no staff record.
// Staff membership is decided by tier, and an unknown tier (base_role NULL on
// a role created before the column existed) counts as personnel.
func TestRoleNeedsStaffRecord(t *testing.T) {
	t.Parallel()

	require.True(t, identityaccess.RoleNeedsStaffRecord(systemRole("admin")))
	require.True(t, identityaccess.RoleNeedsStaffRecord(systemRole("user")))
	require.True(t, identityaccess.RoleNeedsStaffRecord(systemRole("lehrkraft")))
	require.True(t, identityaccess.RoleNeedsStaffRecord(customRole("OGS-Leitung", ptr(identityaccess.BaseRoleAdmin))))
	require.True(t, identityaccess.RoleNeedsStaffRecord(customRole("OGS-Kraft", ptr(identityaccess.BaseRoleUser))))
	require.True(t, identityaccess.RoleNeedsStaffRecord(customRole("Alt-Rolle", nil)))

	require.False(t, identityaccess.RoleNeedsStaffRecord(systemRole("guardian")))
	require.False(t, identityaccess.RoleNeedsStaffRecord(customRole("Sorgeberechtigt", ptr(identityaccess.BaseRoleGuardian))))
	require.False(t, identityaccess.RoleNeedsStaffRecord(nil))
}

// The caregiver upgrade hands out the platform user role for its
// permissions; a school's own role of the same tier does not carry them.
func TestIsPlatformCaregiverRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		roleName string
		want     bool
	}{
		{"teacher role", "teacher", true},
		{"user role", "user", true},
		{"admin role", "admin", false},
		{"uppercase Teacher", "Teacher", true},
		{"whitespace user", " user ", true},
		{"mixed case USER", "USER", true},
		{"empty string", "", false},
		{"unknown role", "moderator", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, identityaccess.IsPlatformCaregiverRole(systemRole(tt.roleName)))
		})
	}

	t.Run("a school's own role of the same tier is not the platform role", func(t *testing.T) {
		t.Parallel()
		require.False(t, identityaccess.IsPlatformCaregiverRole(customRole("OGS-Kraft", ptr(identityaccess.BaseRoleUser))))
	})
	require.False(t, identityaccess.IsPlatformCaregiverRole(nil))
}

// The lehrkraft SYSTEM role is the school-portal role; a school's custom
// role sharing the label is a different role.
func TestIsLehrkraftSystemRole(t *testing.T) {
	t.Parallel()

	require.True(t, identityaccess.IsLehrkraftSystemRole(systemRole("lehrkraft")))
	require.True(t, identityaccess.IsLehrkraftSystemRole(systemRole(" Lehrkraft ")))
	require.False(t, identityaccess.IsLehrkraftSystemRole(&identityaccess.RoleFacts{Name: "lehrkraft"}))
	require.False(t, identityaccess.IsLehrkraftSystemRole(systemRole("user")))
	require.False(t, identityaccess.IsLehrkraftSystemRole(nil))
}
