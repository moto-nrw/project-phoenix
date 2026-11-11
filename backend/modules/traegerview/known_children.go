package traegerview

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The hint on a known child in the review of the holiday registrations
// (#6683, ADR 0214): the Träger-Büro sees that a registered child is known at
// a school of the Träger. The projection names only the school; it links
// nothing and opens no data of the child at the school.
//
// The binding definition: a registered child is known at a school when, of
// all the children the Träger's schools actively manage today by the
// Stichtagszahl rule (#2791), the roster of SchoolFigures, exactly one has
// its first name, last name and date of birth, compared like the
// enrolled-child match of the online enrollment (#6053), and that child is at
// the school. Two or more such children, at one school or at several, give no
// hint; a child the school no longer manages counts for nothing.

// RegisteredChild is a child of a holiday registration as the parents named
// it.
type RegisteredChild struct {
	RegistrationID int64
	ChildID        int64
	FirstName      string
	LastName       string
	BirthDate      calendar.Date
}

// RegisteredChildren are the children of the registrations of one period of
// the Träger that are not cancelled, or of the one registrationID names, at
// Enrollment. It joins the projection's administrative transaction and
// filters on the Träger.
type RegisteredChildren interface {
	RegisteredChildren(ctx context.Context, organizationID, periodID, registrationID int64) ([]RegisteredChild, error)
}

// Namesake is a student profile of a school with the names and the date of
// birth of one registered child; Child is its index in the request.
type Namesake struct {
	Child            int
	SchoolID         int64
	StudentProfileID int64
}

// StudentNamesakes are the namesakes of registered children among the
// student profiles of the given schools, at People Directory. It joins the
// projection's administrative transaction and reads only the given schools.
type StudentNamesakes interface {
	StudentNamesakesByTenant(ctx context.Context, schools []int64, children []RegisteredChild) ([]Namesake, error)
}

// KnownChildrenPorts are the reads behind the hint on a known child.
type KnownChildrenPorts struct {
	Children  RegisteredChildren
	Namesakes StudentNamesakes
}

// KnownChild is a registered child with the one school that knows it.
type KnownChild struct {
	RegistrationID int64
	ChildID        int64
	SchoolID       int64
}
