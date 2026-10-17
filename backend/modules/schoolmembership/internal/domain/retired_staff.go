package domain

// RetiredStaffErasure reports what the erasure of a retired staff member
// removed from the Membership tables (#3963). The membership itself stays as
// a tombstone that records of other people still name.
type RetiredStaffErasure struct {
	StaffID          int64
	PersonID         int64
	Teachers         int64
	GroupAssignments int64
	Guests           int64
	ClassAssignments int64
}
