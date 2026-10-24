package supervisiondashboardhttp

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func TestCallerPermissionsNameTheReadPermissions(t *testing.T) {
	t.Parallel()
	if ScheduleReadPermission != permissions.SchedulesRead || StudentReadPermission != permissions.UsersRead {
		t.Errorf("caller permissions = %q, %q", ScheduleReadPermission, StudentReadPermission)
	}
}
