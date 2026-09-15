package dataimport

import "context"

// OpeningStaff contains only the identity fields used to match an upload row.
type OpeningStaff struct {
	ID              int64
	PersonnelNumber *string
	Person          *OpeningPerson
}

type OpeningPerson struct{ FirstName, LastName string }

// OpeningReferences reads tenant-scoped identities and existing takeovers.
type OpeningReferences struct {
	Staff    func(context.Context) ([]*OpeningStaff, error)
	Hours    func(context.Context) ([]int64, error)
	Vacation func(context.Context, int) ([]int64, error)
}
