package users

import "fmt"

// CaregiverCapabilityBlockedError is returned when caregiver capability cannot
// be removed safely because the account is still referenced operationally.
type CaregiverCapabilityBlockedError struct {
	Reasons []string
}

func (e *CaregiverCapabilityBlockedError) Error() string {
	return "caregiver capability cannot be removed while active bindings exist"
}

// AccountNotAssignedToTenantError is returned when the requested account does
// not belong to the tenant currently being managed.
type AccountNotAssignedToTenantError struct {
	AccountID int64
	TenantID  int64
}

func (e *AccountNotAssignedToTenantError) Error() string {
	return fmt.Sprintf("account %d is not assigned to tenant %d", e.AccountID, e.TenantID)
}
