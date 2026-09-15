package dataimport

import "context"

// Reference is the name-matching projection of a tenant-scoped owner record.
type Reference struct {
	ID   int64
	Name string
}

type ReferenceLookup func(context.Context) ([]Reference, error)

type References struct {
	Groups ReferenceLookup
	Rooms  ReferenceLookup
}
