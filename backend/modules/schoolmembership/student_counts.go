package schoolmembership

import (
	"context"
	"time"
)

// ActiveStudentCounts answers the operator billing report (#2791): how many
// children each school holds right now. School Membership owns the rule of
// what counts: a live membership (not deleted) in status active whose
// enrollment has not ended before the capture instant's Berlin calendar day.
// An active child whose formal start lies after that day counts (immediate
// activation). Pending children whose care has not started, children whose care has ended
// (inactive) and graduates (alumnus) do not count. Schools without an active
// child are missing from the map.
//
// The caller must run it in the administrative transaction, which sees every
// school; inside a tenant transaction it fails instead of counting one school.
type ActiveStudentCounts interface {
	CountActiveStudentsByTenant(context.Context, time.Time) (map[int64]int, error)
}

// ChildQuotaCounts answers the operator school overview (#3568): the
// Kontingentzahl of each school, counted by the same rule and on the same
// Berlin calendar day as the Kinderkontingent check of every membership
// write. Schools without a counted child are missing from the map.
//
// The caller must run it in the administrative transaction, which sees every
// school; inside a tenant transaction it fails instead of counting one school.
type ChildQuotaCounts interface {
	CountChildQuotaByTenant(context.Context) (map[int64]int, error)
}
