package dataimport

import "context"

// StaffInvitation identifies a staff person already created by this import.
// The contract deliberately exposes no operator bypass or caregiver override.
type StaffInvitation struct {
	Email            string
	RoleID           int64
	TenantID         int64
	FirstName        *string
	LastName         *string
	Position         *string
	PersonID         *int64
	CreatedBy        int64
	SchoolName       string
	ActorPermissions []string
}

type StaffInviter interface {
	InviteStaff(context.Context, StaffInvitation) error
}
