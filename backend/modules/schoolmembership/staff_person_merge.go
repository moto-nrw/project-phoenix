package schoolmembership

import "context"

// StaffPersonMerge preserves employment IDs and every record referring to them.
// It joins an active and a retired employment under the retained person.
type StaffPersonReassignment struct {
	SchoolID       int64
	StaffID        int64
	DonorPersonID  int64
	KeeperPersonID int64
}

type StaffPersonMerge interface {
	StaffMergeMemberships(context.Context, int64, bool) ([]Staff, error)
	ReassignStaffPerson(context.Context, StaffPersonReassignment) error
}
