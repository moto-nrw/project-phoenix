package schoolmembership

import "context"

// ActiveStudentCounts answers the operator billing report (#2791): how many
// children each school holds right now. School Membership owns the rule of
// what counts: a live membership (not deleted) in status active. Pending
// children whose care has not started, children whose care has ended
// (inactive) and graduates (alumnus) do not count. Schools without an active
// child are missing from the map.
//
// The caller must run it in the administrative transaction, which sees every
// school; inside a tenant transaction it fails instead of counting one school.
type ActiveStudentCounts interface {
	CountActiveStudentsByTenant(context.Context) (map[int64]int, error)
}
