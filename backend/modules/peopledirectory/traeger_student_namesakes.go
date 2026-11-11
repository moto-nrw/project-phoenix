package peopledirectory

import "context"

// StudentNamesake is a student profile of a school with the names and the
// date of birth of one asked identity; Identity is its index in the request.
type StudentNamesake struct {
	Identity         int
	SchoolID         int64
	StudentProfileID int64
}

// StudentNamesakes answers the Träger-Büro's hint on a holiday registration
// (#6683, ADR 0214): the student profiles of the given schools whose person
// has the first name, last name and date of birth of an identity, compared
// like the enrolled-child match of the online enrollment (#6053). It names no
// person and says nothing about the membership; the caller decides which
// profiles count.
//
// The caller must run it in the administrative transaction; it reads only
// the given schools.
type StudentNamesakes interface {
	StudentNamesakesByTenant(ctx context.Context, schools []int64, identities []StudentIdentity) ([]StudentNamesake, error)
}
