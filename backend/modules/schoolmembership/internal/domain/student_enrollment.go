package domain

import "fmt"

// ChildQuotaReachedError refuses a write that would raise the Kontingentzahl
// above the Kinderkontingent (#3567). Booked is the Kinderkontingent,
// Occupied the Kontingentzahl before the write, Requested what it would add.
type ChildQuotaReachedError struct {
	Booked    int
	Occupied  int
	Requested int
}

func (e *ChildQuotaReachedError) Error() string {
	return fmt.Sprintf("child quota reached: %d of %d places occupied, %d requested", e.Occupied, e.Booked, e.Requested)
}

// CheckChildQuota decides a counting write: it is refused when it adds
// children and the Kontingentzahl after it exceeds the Kinderkontingent. A
// write that adds nobody passes even when the school is already above its
// Kinderkontingent, so a lowered contract never blocks existing children.
func CheckChildQuota(limit, before, after int) error {
	if after <= before || after <= limit {
		return nil
	}
	return &ChildQuotaReachedError{Booked: limit, Occupied: before, Requested: after - before}
}

// ChildQuotaUsage is the Kinderkontingent (Booked) next to the Kontingentzahl
// (Occupied) of one school on one day (#3569).
type ChildQuotaUsage struct {
	Booked   int
	Occupied int
}

// StudentEnrollment uses the public profile ID, never the membership row ID.
type StudentEnrollment struct {
	StudentID     int64
	GroupID       *int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}
