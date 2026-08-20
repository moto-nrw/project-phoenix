package users

import (
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestCaregiverCapabilityBlockedError_Error(t *testing.T) {
	t.Parallel()

	err := &CaregiverCapabilityBlockedError{
		Reasons: []userModels.CaregiverCapabilityBlockerCode{
			userModels.CaregiverCapabilityBlockerActiveGroupSupervisions,
		},
	}

	if got := err.Error(); got != "caregiver capability cannot be removed while active bindings exist" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestAccountNotAssignedToTenantError_Error(t *testing.T) {
	t.Parallel()

	err := &AccountNotAssignedToTenantError{
		AccountID: 42,
		TenantID:  7,
	}

	if got := err.Error(); got != "account 42 is not assigned to tenant 7" {
		t.Fatalf("Error() = %q", got)
	}
}
