package schedule

import "testing"

// Pure model tests for the care-schedule change-request lifecycle helpers
// (#1803). No database — these guard the state machine and table binding that
// the repository, service, and staff review queue all depend on.

func TestCareScheduleChangeRequest_IsTerminal(t *testing.T) {
	cases := []struct {
		status   string
		terminal bool
	}{
		{CareRequestStatusPending, false},
		{CareRequestStatusApproved, true},
		{CareRequestStatusRejected, true},
		{CareRequestStatusWithdrawn, true},
		{"", false},        // an unset status is treated as still open
		{"garbage", false}, // unknown statuses must not read as decided
	}
	for _, tc := range cases {
		req := &CareScheduleChangeRequest{Status: tc.status}
		if got := req.IsTerminal(); got != tc.terminal {
			t.Errorf("IsTerminal(status=%q) = %v, want %v", tc.status, got, tc.terminal)
		}
	}
}

func TestCareScheduleChangeRequest_BeforeAppendModel_IgnoresNonWriteQueries(t *testing.T) {
	req := &CareScheduleChangeRequest{}
	// A nil / non-write query type must be tolerated without panicking and
	// return no error (the ModelTableExpr rewrite only applies to
	// insert/update/delete queries).
	if err := req.BeforeAppendModel(nil); err != nil {
		t.Errorf("BeforeAppendModel(nil) = %v, want nil", err)
	}
	if err := req.BeforeAppendModel("not a query"); err != nil {
		t.Errorf("BeforeAppendModel(non-query) = %v, want nil", err)
	}
}
