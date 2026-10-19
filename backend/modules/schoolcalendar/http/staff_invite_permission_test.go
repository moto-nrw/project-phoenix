package staffcalendarhttp

import "testing"

func TestStaffInvitePermissionIsPersonalCalendarAccess(t *testing.T) {
	t.Parallel()
	if StaffInvitePermission != "calendar:own" {
		t.Errorf("StaffInvitePermission = %q", StaffInvitePermission)
	}
}
