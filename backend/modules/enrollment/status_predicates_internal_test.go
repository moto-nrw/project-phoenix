package enrollment

import "testing"

func TestSettledEnrollmentCleanupResultDeletedCountsRequestsAndInvites(t *testing.T) {
	t.Parallel()
	if got := (SettledEnrollmentCleanupResult{DeletedRequests: 4, DeletedLateInvites: 3, RetentionDays: 30}).Deleted(); got != 7 {
		t.Errorf("Deleted() = %d, want 7", got)
	}
}
