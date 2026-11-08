package enrollment

import "context"

// OpenEnrollmentChildStatuses are the statuses of a child of the online
// enrollment that still waits for a decision of the school: the ones the
// school's own list counts as „Offen“.
func OpenEnrollmentChildStatuses() []string {
	return []string{ChildStatusSubmitted, ChildStatusUnderReview, ChildStatusPendingAdminReview}
}

// OpenEnrollments answers the Träger dashboard (#4675): how many children of
// the online enrollment wait for a decision at each school, counted as the
// school's own enrollment list counts „Offen“. A child of a withdrawn request
// does not wait. The map is keyed by school and carries counts only; a school
// without a waiting child is missing.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type OpenEnrollments interface {
	OpenEnrollmentChildrenByTenant(ctx context.Context, schools []int64) (map[int64]int, error)
}
