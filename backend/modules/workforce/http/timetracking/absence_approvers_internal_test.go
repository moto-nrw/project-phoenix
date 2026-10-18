package timetracking

import "testing"

func TestAbsenceApproversHoldVacationApprove(t *testing.T) {
	t.Parallel()
	if AbsenceApproverPermission != "vacation:approve" {
		t.Errorf("AbsenceApproverPermission = %q", AbsenceApproverPermission)
	}
}
