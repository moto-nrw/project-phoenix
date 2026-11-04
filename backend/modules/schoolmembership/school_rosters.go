package schoolmembership

import (
	"context"
	"time"
)

// SchoolRosters answers the Träger's view of a school (#5686): who a school
// counts on one day, as IDs without names. School Membership owns both rules.
// Each map is keyed by school; a school without an entry is missing from it.
//
// The caller must run them in the administrative transaction; they read only
// the given schools.
type SchoolRosters interface {
	// ActiveStudentMembershipsByTenant maps each counted child to its
	// membership, by the rule of ActiveStudentCounts (#2791).
	ActiveStudentMembershipsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64]map[int64]int64, error)
	// ActiveStaffIDsByTenant lists the live staff members without a current
	// external caregiver record.
	ActiveStaffIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error)
}
