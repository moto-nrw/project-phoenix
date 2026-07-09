// Package schedule — Betreuungsplan capacity computation (issue #1838).
//
// Computes the Betreuungsschlüssel-derived staffing requirement for a
// materialized Betreuungsplan block (activity instance). The list endpoints
// (instances_list.go, templates_list.go) already have instance_staff/
// instance_students rows loaded per row, so they call RequiredStaffForChildren
// directly on the counts they already hold instead of a dedicated service —
// that would mean an extra DB round-trip per list request for no benefit.
package schedule

// RequiredStaffForChildren returns the number of staff required to supervise
// childrenCount children at the given ratio (max children per staff member),
// rounded up. ratio < 1 is defensively clamped to 1 so a corrupted override
// can never divide by zero or invert the relationship; the registry
// Validation (1-30) already prevents this in practice.
func RequiredStaffForChildren(childrenCount, ratio int) int {
	if childrenCount <= 0 {
		return 0
	}
	if ratio < 1 {
		ratio = 1
	}
	return (childrenCount + ratio - 1) / ratio
}
