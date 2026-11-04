package parentbilling

import (
	"context"
	"errors"
)

// Siblings from separate holiday registrations (#5628, ADR 0115 addendum).
// Parents register siblings in two forms, or a fourth child in a second
// one. The Träger-Büro merges such registrations of one period into a
// sibling group by click, never automatically. The sibling rule then ranks
// the children of every registration of the group that is not cancelled as
// if they were one registration: the highest contribution first, on a tie
// the registration sent earlier, then the order in the form. Every
// registration keeps only the lines of its own children, and its invoice
// and letter name only its own family. A merge and a split compute the
// frozen contribution of each confirmed registration of the group again
// and store it as a new version, until an issued invoice bills one of them.

// Actions of a sibling change in the audit trail of a registration.
const (
	HolidaySiblingsMerged = "sibling_merge"
	HolidaySiblingsSplit  = "sibling_split"
)

var (
	// ErrInvalidHolidaySiblingGroup reports a merge of fewer than two
	// registrations.
	ErrInvalidHolidaySiblingGroup = errors.New("a sibling group needs at least two holiday registrations")
	// ErrHolidaySiblingGroupNotFound reports a split of a registration that
	// belongs to no sibling group.
	ErrHolidaySiblingGroupNotFound = errors.New("the holiday registration belongs to no sibling group")
	// ErrHolidaySiblingsInvoiced reports a merge or split of a group in
	// which an issued invoice already bills a registration.
	ErrHolidaySiblingsInvoiced = errors.New("an issued invoice bills a registration of the sibling group")
)

// HolidaySiblingGroup is a sibling group of a period by its registrations,
// by number.
type HolidaySiblingGroup struct {
	ID              int64   `json:"id"`
	RegistrationIDs []int64 `json:"registration_ids"`
}

// HolidaySiblingMember is one registration of a sibling group.
type HolidaySiblingMember struct {
	GroupID        int64
	RegistrationID int64
}

// HolidaySiblingChild names a child of another registration of the group.
type HolidaySiblingChild struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// HolidaySibling is another registration of the group of a registration,
// as the Träger-Büro sees it in the registration.
type HolidaySibling struct {
	RegistrationID int64                 `json:"registration_id"`
	Status         string                `json:"status"`
	Children       []HolidaySiblingChild `json:"children"`
}

// HolidaySiblingGroups lets the Träger-Büro merge and split siblings of one
// period. Every operation runs in the Träger transaction of the session's
// Träger, which the caller opens. A merge and a split read every
// registration of the group through Enrollment, which records the read,
// and record the change in the audit trail of each registration.
type HolidaySiblingGroups interface {
	// ListHolidaySiblingGroups lists the groups of the period.
	ListHolidaySiblingGroups(ctx context.Context, reviewer HolidayReviewer, periodID int64) ([]HolidaySiblingGroup, error)
	// MergeHolidaySiblings merges the registrations, with the groups they
	// already belong to, into one group. Fewer than two registrations is
	// ErrInvalidHolidaySiblingGroup, a cancelled one
	// ErrHolidayRegistrationStateConflict, a registration of another period
	// ErrHolidayRegistrationNotFound, an issued invoice in the group
	// ErrHolidaySiblingsInvoiced.
	MergeHolidaySiblings(ctx context.Context, reviewer HolidayReviewer, periodID int64, registrationIDs []int64) (HolidaySiblingGroup, error)
	// SplitHolidaySibling takes the registration out of its group; a group
	// with one registration left goes. A registration without a group is
	// ErrHolidaySiblingGroupNotFound, an issued invoice in the group
	// ErrHolidaySiblingsInvoiced.
	SplitHolidaySibling(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) error
}
