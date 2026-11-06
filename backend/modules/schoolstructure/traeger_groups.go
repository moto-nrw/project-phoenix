package schoolstructure

import "context"

// NamedGroup is an OGS group of a school with its name.
type NamedGroup struct {
	ID   int64
	Name string
}

// TraegerGroupNames answers the Träger's occupancy of a school (#4676): the
// OGS groups of each school with their names, ordered by name. The map is
// keyed by school; a school without a group is missing.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type TraegerGroupNames interface {
	GroupsByTenant(ctx context.Context, schools []int64) (map[int64][]NamedGroup, error)
}
