package ports

import (
	"context"
)

// CaregiverAccount is the account a caregiver capability belongs to.
type CaregiverAccount struct {
	ID    int64
	Email string
}

// CaregiverIdentity supplies the account and school-role facts needed to
// enable or disable a caregiver profile. Calls join the caller's transaction.
type CaregiverIdentity interface {
	FindCaregiverAccount(context.Context, int64) (*CaregiverAccount, error)
	HasActiveSchoolMembership(context.Context, int64) (bool, error)
	ListSchoolAccountRoleNames(context.Context, int64) ([]string, error)
	FindSystemRoleID(context.Context, string) (int64, bool, error)
}
