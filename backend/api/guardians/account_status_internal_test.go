package guardians

import "testing"

// TestGuardianAccountStatus locks the per-child derivation of the staff-facing
// account status: an account only counts as "active" for a child when the
// students_guardians link actually grants parent_portal.access, and an open
// invitation (e.g. a pending-approval role-upgrade request) outranks the
// actionable "active_no_access" state (#2172).
func TestGuardianAccountStatus(t *testing.T) {
	cases := []struct {
		name                                       string
		hasAccount, hasPortalAccess, invitePending bool
		want                                       string
	}{
		{"account with access", true, true, false, "active"},
		{"account without access on this child", true, false, false, "active_no_access"},
		{"account without access, pending upgrade approval", true, false, true, "pending"},
		{"no account, open invitation", false, false, true, "pending"},
		{"no account, no invitation", false, false, false, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := guardianAccountStatus(tc.hasAccount, tc.hasPortalAccess, tc.invitePending)
			if got != tc.want {
				t.Fatalf("guardianAccountStatus(%v, %v, %v) = %q, want %q",
					tc.hasAccount, tc.hasPortalAccess, tc.invitePending, got, tc.want)
			}
		})
	}
}
