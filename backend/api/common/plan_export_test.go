package common

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func TestCanExportInternalPlanRequiresScheduleManagement(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		permissions []string
		want        bool
	}{
		{name: "read-only planner", permissions: []string{permissions.SchedulesRead, permissions.UsersRead}, want: false},
		{name: "schedule manager", permissions: []string{permissions.SchedulesManage}, want: true},
		{name: "platform administrator", permissions: []string{"admin:*"}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanExportInternalPlan(tc.permissions); got != tc.want {
				t.Fatalf("CanExportInternalPlan() = %v, want %v", got, tc.want)
			}
		})
	}
}
