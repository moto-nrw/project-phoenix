// Package schoolstructure is the public School Structure capability. It owns
// education.groups and answers class and group structure questions for every
// other owner without leaking the table into their repositories.
package schoolstructure

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
)

var (
	ErrGroupNotFound  = errors.New("group not found")
	ErrInvalidGroup   = errors.New("invalid group")
	ErrInvalidStudent = errors.New("invalid student")
)

type Group = contract.Group

// Query is the read seam every foreign reader of education.groups uses.
type Query interface {
	FindGroup(context.Context, int64) (Group, error)
	ListGroupsByID(context.Context, []int64) ([]Group, error)
}

type Capability interface {
	Query
	GroupListing
}

// GroupListing supplies bounded name-matching candidates to import workflows.
type GroupListing interface {
	ListGroups(context.Context, int) ([]Group, error)
}

// StudentTransitionHistory is the School Structure half of a permanent child
// deletion (#2710): education.grade_transition_history keeps a denormalized
// copy of the child's name without a foreign key, so the owner counts and
// anonymizes those rows inside the deletion's transaction.
type StudentTransitionHistory interface {
	// CountStudentTransitionHistory counts the ledger rows that still carry
	// the child's name; anonymized rows are retained history, not impact.
	CountStudentTransitionHistory(context.Context, int64) (int, error)
	// AnonymizeStudentTransitionHistory replaces the stored name with the
	// placeholder and clears the stored RFID tag on every ledger row of the
	// child. It returns the number of rows changed.
	AnonymizeStudentTransitionHistory(context.Context, int64) (int64, error)
}

// PurgedStudentPlaceholder is the name the ledger keeps for a deleted child.
const PurgedStudentPlaceholder = "Gelöschtes Kind"

// UnknownGroupName labels a group the caller cannot see, such as the group
// of a supervision that blocks removing a caregiver.
const UnknownGroupName = "Unbekannte Gruppe"

// Owner is the whole School Structure surface the composition hands to the
// roots: the group reads, the child-deletion ledger half, the grade
// transitions and their retention.
type Owner interface {
	Capability
	StudentTransitionHistory
	TransitionCapability
	GradeTransitionRetention
}

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrGroupNotFound), errors.Is(err, ErrTransitionNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidGroup), errors.Is(err, ErrInvalidTransition),
		errors.Is(err, ErrInvalidStaff), errors.Is(err, ErrInvalidDay):
		return "invalid"
	case errors.Is(err, ErrTransitionStateConflict):
		return "conflict"
	default:
		return "internal_error"
	}
}
