package users

import "testing"

func TestCaregiverCapabilityBlockedError_Error(t *testing.T) {
	err := &CaregiverCapabilityBlockedError{
		Reasons: []string{"active supervision"},
	}

	if got := err.Error(); got != "caregiver capability cannot be removed while active bindings exist" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestAccountNotAssignedToTenantError_Error(t *testing.T) {
	err := &AccountNotAssignedToTenantError{
		AccountID: 42,
		TenantID:  7,
	}

	if got := err.Error(); got != "account 42 is not assigned to tenant 7" {
		t.Fatalf("Error() = %q", got)
	}
}
