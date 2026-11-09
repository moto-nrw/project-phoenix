package peopledirectory

import "context"

// PersonBatches creates the persons of many import rows at once (#5552, ADR
// 0067 addendum of 2026-11-08, step 5). It is a leaf command (#4538) and
// stays outside Capability and Owner; the data import binds it on its own.
type PersonBatches interface {
	// CreatePersons normalizes and validates every input like CreatePerson
	// and creates every person or none, with the columns CreatePerson writes,
	// under the same shared staff-person-merge lock. The result is
	// index-aligned with the input; a card or account already taken fails the
	// whole call with ErrTagConflict or ErrAccountConflict.
	CreatePersons(context.Context, []CreatePerson) ([]Person, error)
}
