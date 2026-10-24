package enrollment

import "testing"

func TestApprovalAndDayModePredicates(t *testing.T) {
	t.Parallel()
	if !(CareExitApplicationLink{Status: ChildStatusApproved}).IsApproved() || (CareExitApplicationLink{Status: ChildStatusRejected}).IsApproved() {
		t.Error("only an approved application link is approved")
	}
	if !(&RequestChild{Status: ChildStatusApproved}).IsApproved() || (&RequestChild{Status: ChildStatusWithdrawn}).IsApproved() {
		t.Error("only an approved child is approved")
	}
	if !(&CareOffering{DaysOfWeekMode: CareOfferingDaysFixed}).HasFixedDays() || (&CareOffering{DaysOfWeekMode: ""}).HasFixedDays() {
		t.Error("only the fixed mode books every available day")
	}
}

func TestSettledEnrollmentCleanupResultDeletedCountsRequestsAndInvites(t *testing.T) {
	t.Parallel()
	if got := (SettledEnrollmentCleanupResult{DeletedRequests: 4, DeletedLateInvites: 3, RetentionDays: 30}).Deleted(); got != 7 {
		t.Errorf("Deleted() = %d, want 7", got)
	}
}
